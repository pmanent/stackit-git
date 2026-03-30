// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package runner

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	actions_model "forgejo.org/models/actions"
	auth_model "forgejo.org/models/auth"
	"forgejo.org/modules/log"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"

	"connectrpc.com/connect"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	uuidHeaderKey  = "x-runner-uuid"
	tokenHeaderKey = "x-runner-token"
)

// Interceptor that ensures a valid HTTP Status code exists for gRPC error
// statuses that were raised by the next interceptor.
var withHttpErrorStatusCodes = connect.UnaryInterceptorFunc(func(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		res, err := next(ctx, request)

		// If no error occurred, return the response as is
		if err == nil {
			return res, nil
		}

		// No need to re-map connect.Error since they already provide a status code
		if _, ok := err.(*connect.Error); ok {
			return res, err
		}

		// connect.Error provide HTTP status codes for gRPC error statuses, so we
		// can convert error status to connect.Error
		if status, ok := status.FromError(err); ok {
			// In both name and semantics, these connect.Code's match gRPC status
			// codes, and there are no user-defined codes, so we can simply cast.
			return res, connect.NewError(connect.Code(status.Code()), status.Err())
		}

		// Some other error occured that is not a gRPC error status which
		// necessarily implies an internal server error
		return res, connect.NewError(connect.Code(codes.Internal), err)
	}
})

var withRunner = connect.UnaryInterceptorFunc(func(unaryFunc connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, request connect.AnyRequest) (connect.AnyResponse, error) {
		methodName := getMethodName(request)
		if methodName == "Register" {
			return unaryFunc(ctx, request)
		}
		uuid := request.Header().Get(uuidHeaderKey)
		token := request.Header().Get(tokenHeaderKey)

		runner, err := actions_model.GetRunnerByUUID(ctx, uuid)
		if err != nil {
			if errors.Is(err, util.ErrNotExist) {
				return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unregistered runner"))
			}
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if subtle.ConstantTimeCompare([]byte(runner.TokenHash), []byte(auth_model.HashToken(token, runner.TokenSalt))) != 1 {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unregistered runner"))
		}

		cols := []string{"last_online"}
		runner.LastOnline = timeutil.TimeStampNow()
		if methodName == "UpdateTask" || methodName == "UpdateLog" {
			runner.LastActive = timeutil.TimeStampNow()
			cols = append(cols, "last_active")
		}
		if err := actions_model.UpdateRunner(ctx, runner, cols...); err != nil {
			log.Error("can't update runner status: %v", err)
		}

		ctx = context.WithValue(ctx, runnerCtxKey{}, runner)
		return unaryFunc(ctx, request)
	}
})

func getMethodName(req connect.AnyRequest) string {
	splits := strings.Split(req.Spec().Procedure, "/")
	if len(splits) > 0 {
		return splits[len(splits)-1]
	}
	return ""
}

type runnerCtxKey struct{}

func GetRunner(ctx context.Context) *actions_model.ActionRunner {
	if v := ctx.Value(runnerCtxKey{}); v != nil {
		if r, ok := v.(*actions_model.ActionRunner); ok {
			return r
		}
	}
	return nil
}
