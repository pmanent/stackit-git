// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	go_context "context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	asymkey_model "forgejo.org/models/asymkey"
	"forgejo.org/models/auth"
	org_model "forgejo.org/models/organization"
	user_model "forgejo.org/models/user"
	auth_module "forgejo.org/modules/auth"
	pwd "forgejo.org/modules/auth/password" // >>> @@@ STACKIT CODE @@@ User Story 44186
	"forgejo.org/modules/base"
	"forgejo.org/modules/container"
	"forgejo.org/modules/json"
	"forgejo.org/modules/jwtx"
	"forgejo.org/modules/log"
	"forgejo.org/modules/optional"
	"forgejo.org/modules/setting"
	"forgejo.org/modules/timeutil"
	"forgejo.org/modules/util"
	"forgejo.org/modules/validation"
	"forgejo.org/modules/web"
	"forgejo.org/modules/web/middleware"
	source_service "forgejo.org/services/auth/source"
	"forgejo.org/services/auth/source/oauth2"
	"forgejo.org/services/context"
	"forgejo.org/services/externalaccount"
	"forgejo.org/services/forms"
	remote_service "forgejo.org/services/remote"
	user_service "forgejo.org/services/user"

	"code.forgejo.org/go-chi/binding"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/markbates/goth"
	"github.com/markbates/goth/gothic"
	"github.com/markbates/goth/providers/fitbit"
	"github.com/markbates/goth/providers/openidConnect"
	"github.com/markbates/goth/providers/zoom"
	"github.com/stackitcloud/stackit-sdk-go/core/config"              // >>> @@@ STACKIT CODE @@@ User Story 44186
	"github.com/stackitcloud/stackit-sdk-go/services/authorization"   // >>> @@@ STACKIT CODE @@@ User Story 44186
	"github.com/stackitcloud/stackit-sdk-go/services/resourcemanager" // >>> @@@ STACKIT CODE @@@ User Story 44186
	go_oauth2 "golang.org/x/oauth2"
)

const (
	tplGrantAccess base.TplName = "user/auth/grant"
	tplGrantError  base.TplName = "user/auth/grant_error"

	// >>> @@@ STACKIT CODE @@@
	// User Story 44186
	// Taken from the default settings
	DEFAULTPWDLENGTH = 23

	ParentTypeOrg = "ORGANIZATION" // Parent type organization returned by the api
	// <<< @@@ STACKIT CODE @@@
)

// TODO move error and responses to SDK or models

// AuthorizeErrorCode represents an error code specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2.1
type AuthorizeErrorCode string

const (
	// ErrorCodeInvalidRequest represents the according error in RFC 6749
	ErrorCodeInvalidRequest AuthorizeErrorCode = "invalid_request"
	// ErrorCodeUnauthorizedClient represents the according error in RFC 6749
	ErrorCodeUnauthorizedClient AuthorizeErrorCode = "unauthorized_client"
	// ErrorCodeAccessDenied represents the according error in RFC 6749
	ErrorCodeAccessDenied AuthorizeErrorCode = "access_denied"
	// ErrorCodeUnsupportedResponseType represents the according error in RFC 6749
	ErrorCodeUnsupportedResponseType AuthorizeErrorCode = "unsupported_response_type"
	// ErrorCodeInvalidScope represents the according error in RFC 6749
	ErrorCodeInvalidScope AuthorizeErrorCode = "invalid_scope"
	// ErrorCodeServerError represents the according error in RFC 6749
	ErrorCodeServerError AuthorizeErrorCode = "server_error"
	// ErrorCodeTemporaryUnavailable represents the according error in RFC 6749
	ErrorCodeTemporaryUnavailable AuthorizeErrorCode = "temporarily_unavailable"
)

// AuthorizeError represents an error type specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2.1
type AuthorizeError struct {
	ErrorCode        AuthorizeErrorCode `json:"error" form:"error"`
	ErrorDescription string
	State            string
}

// Error returns the error message
func (err AuthorizeError) Error() string {
	return fmt.Sprintf("%s: %s", err.ErrorCode, err.ErrorDescription)
}

// AccessTokenErrorCode represents an error code specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
type AccessTokenErrorCode string

const (
	// AccessTokenErrorCodeInvalidRequest represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidRequest AccessTokenErrorCode = "invalid_request"
	// AccessTokenErrorCodeInvalidClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidClient = "invalid_client"
	// AccessTokenErrorCodeInvalidGrant represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidGrant = "invalid_grant"
	// AccessTokenErrorCodeUnauthorizedClient represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnauthorizedClient = "unauthorized_client"
	// AccessTokenErrorCodeUnsupportedGrantType represents an error code specified in RFC 6749
	AccessTokenErrorCodeUnsupportedGrantType = "unsupported_grant_type"
	// AccessTokenErrorCodeInvalidScope represents an error code specified in RFC 6749
	AccessTokenErrorCodeInvalidScope = "invalid_scope"
)

// AccessTokenErrorResponse represents an error response specified in RFC 6749
// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
type AccessTokenErrorResponse struct {
	ErrorCode        AccessTokenErrorCode `json:"error" form:"error"`
	ErrorDescription string               `json:"error_description"`
}

// errCallback represents a oauth2 callback error
type errCallback struct {
	Code        string
	Description string
}

func (err errCallback) Error() string {
	return err.Description
}

func isOIDCSilentAuthFailure(code string) bool {
	switch code {
	// access_denied is non-standard for prompt=none but emitted by Keycloak and some Azure AD configurations.
	case "login_required", "interaction_required", "account_selection_required", "consent_required", "access_denied":
		return true
	}
	return false
}

// TokenType specifies the kind of token
type TokenType string

const (
	// TokenTypeBearer represents a token type specified in RFC 6749
	TokenTypeBearer TokenType = "bearer"
	// TokenTypeMAC represents a token type specified in RFC 6749
	TokenTypeMAC = "mac"
)

// AccessTokenResponse represents a successful access token response
// https://datatracker.ietf.org/doc/html/rfc6749#section-4.2.2
type AccessTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	TokenType    TokenType `json:"token_type"`
	ExpiresIn    int64     `json:"expires_in"`
	RefreshToken string    `json:"refresh_token"`
	IDToken      string    `json:"id_token,omitempty"`
}

func newAccessTokenResponse(ctx go_context.Context, grant *auth.OAuth2Grant, serverKey, clientKey jwtx.SigningKey) (*AccessTokenResponse, *AccessTokenErrorResponse) {
	if setting.OAuth2.InvalidateRefreshTokens {
		if err := grant.IncreaseCounter(ctx); err != nil {
			return nil, &AccessTokenErrorResponse{
				ErrorCode:        AccessTokenErrorCodeInvalidGrant,
				ErrorDescription: "cannot increase the grant counter",
			}
		}
	}
	// generate access token to access the API
	expirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.AccessTokenExpirationTime)
	accessToken := &oauth2.Token{
		GrantID: grant.ID,
		Type:    oauth2.TypeAccessToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationDate.AsTime()),
		},
	}
	signedAccessToken, err := accessToken.SignToken(serverKey)
	if err != nil {
		return nil, &AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate refresh token to request an access token after it expired later
	refreshExpirationDate := timeutil.TimeStampNow().Add(setting.OAuth2.RefreshTokenExpirationTime * 60 * 60).AsTime()
	refreshToken := &oauth2.Token{
		GrantID: grant.ID,
		Counter: grant.Counter,
		Type:    oauth2.TypeRefreshToken,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(refreshExpirationDate),
		},
	}
	signedRefreshToken, err := refreshToken.SignToken(serverKey)
	if err != nil {
		return nil, &AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot sign token",
		}
	}

	// generate OpenID Connect id_token
	signedIDToken := ""
	if grant.ScopeContains("openid") {
		app, err := auth.GetOAuth2ApplicationByID(ctx, grant.ApplicationID)
		if err != nil {
			return nil, &AccessTokenErrorResponse{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot find application",
			}
		}
		user, err := user_model.GetUserByID(ctx, grant.UserID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				return nil, &AccessTokenErrorResponse{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot find user",
				}
			}
			log.Error("Error loading user: %v", err)
			return nil, &AccessTokenErrorResponse{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "server error",
			}
		}

		idToken := &oauth2.OIDCToken{
			RegisteredClaims: jwt.RegisteredClaims{
				ExpiresAt: jwt.NewNumericDate(expirationDate.AsTime()),
				Issuer:    strings.TrimSuffix(setting.AppURL, "/"),
				Audience:  []string{app.ClientID},
				Subject:   fmt.Sprint(grant.UserID),
			},
			Nonce: grant.Nonce,
		}
		if grant.ScopeContains("profile") {
			idToken.Name = user.DisplayName()
			idToken.PreferredUsername = user.Name
			idToken.Profile = user.HTMLURL()
			idToken.Picture = user.AvatarLink(ctx)
			idToken.Website = user.Website
			idToken.Locale = user.Language
			idToken.UpdatedAt = user.UpdatedUnix
		}
		if grant.ScopeContains("email") {
			idToken.Email = user.Email
			idToken.EmailVerified = user.IsActive
		}
		if grant.ScopeContains("groups") {
			onlyPublicGroups := ifOnlyPublicGroups(grant.Scope)

			groups, err := getOAuthGroupsForUser(ctx, user, onlyPublicGroups)
			if err != nil {
				log.Error("Error getting groups: %v", err)
				return nil, &AccessTokenErrorResponse{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "server error",
				}
			}
			idToken.Groups = groups
		}

		signedIDToken, err = idToken.SignToken(clientKey)
		if err != nil {
			return nil, &AccessTokenErrorResponse{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "cannot sign token",
			}
		}
	}

	return &AccessTokenResponse{
		AccessToken:  signedAccessToken,
		TokenType:    TokenTypeBearer,
		ExpiresIn:    setting.OAuth2.AccessTokenExpirationTime,
		RefreshToken: signedRefreshToken,
		IDToken:      signedIDToken,
	}, nil
}

type userInfoResponse struct {
	Sub      string   `json:"sub"`
	Name     string   `json:"name"`
	Username string   `json:"preferred_username"`
	Email    string   `json:"email"`
	Picture  string   `json:"picture"`
	Groups   []string `json:"groups,omitempty"`
}

func ifOnlyPublicGroups(scopes string) bool {
	scopes = strings.ReplaceAll(scopes, ",", " ")
	scopesList := strings.FieldsSeq(scopes)
	for scope := range scopesList {
		if scope == "all" || scope == "read:organization" || scope == "read:admin" {
			return false
		}
	}
	return true
}

// InfoOAuth manages request for userinfo endpoint
func InfoOAuth(ctx *context.Context) {
	hasGrantScopes, grantScopes := ctx.Authentication.OAuth2GrantScopes().Get()

	if ctx.Doer == nil || !hasGrantScopes {
		ctx.Resp.Header().Set("WWW-Authenticate", `Bearer realm=""`)
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	response := &userInfoResponse{
		Sub:      fmt.Sprint(ctx.Doer.ID),
		Name:     ctx.Doer.DisplayName(),
		Username: ctx.Doer.Name,
		Email:    ctx.Doer.Email,
		Picture:  ctx.Doer.AvatarLink(ctx),
	}

	onlyPublicGroups := ifOnlyPublicGroups(grantScopes)
	groups, err := getOAuthGroupsForUser(ctx, ctx.Doer, onlyPublicGroups)
	if err != nil {
		ctx.ServerError("Oauth groups for user", err)
		return
	}
	response.Groups = groups

	ctx.JSON(http.StatusOK, response)
}

// returns a list of "org" and "org:team" strings,
// that the given user is a part of.
func getOAuthGroupsForUser(ctx go_context.Context, user *user_model.User, onlyPublicGroups bool) ([]string, error) {
	orgs, err := org_model.GetUserOrgsList(ctx, user)
	if err != nil {
		return nil, fmt.Errorf("GetUserOrgList: %w", err)
	}

	var groups []string
	for _, org := range orgs {
		if setting.OAuth2.EnableAdditionalGrantScopes {
			if onlyPublicGroups {
				public, err := org_model.IsPublicMembership(ctx, org.ID, user.ID)
				if !public && err == nil {
					continue
				}
			}
		}

		groups = append(groups, org.Name)
		teams, err := org.LoadTeams(ctx)
		if err != nil {
			return nil, fmt.Errorf("LoadTeams: %w", err)
		}
		for _, team := range teams {
			if team.IsMember(ctx, user.ID) {
				groups = append(groups, org.Name+":"+team.LowerName)
			}
		}
	}
	return groups, nil
}

func parseBasicAuth(ctx *context.Context) (username, password string, err error) {
	authHeader := ctx.Req.Header.Get("Authorization")
	if authType, authData, ok := strings.Cut(authHeader, " "); ok && strings.EqualFold(authType, "Basic") {
		return base.BasicAuthDecode(authData)
	}
	return "", "", errors.New("invalid basic authentication")
}

// IntrospectOAuth introspects an oauth token
func IntrospectOAuth(ctx *context.Context) {
	clientIDValid := false
	if clientID, clientSecret, err := parseBasicAuth(ctx); err == nil {
		app, err := auth.GetOAuth2ApplicationByClientID(ctx, clientID)
		if err != nil && !auth.IsErrOauthClientIDInvalid(err) {
			// this is likely a database error; log it and respond without details
			log.Error("Error retrieving client_id: %v", err)
			ctx.Error(http.StatusInternalServerError)
			return
		}
		clientIDValid = err == nil && app.ValidateClientSecret([]byte(clientSecret))
	}
	if !clientIDValid {
		ctx.Resp.Header().Set("WWW-Authenticate", `Basic realm=""`)
		ctx.PlainText(http.StatusUnauthorized, "no valid authorization")
		return
	}

	var response struct {
		Active   bool   `json:"active"`
		Scope    string `json:"scope,omitempty"`
		Username string `json:"username,omitempty"`
		jwt.RegisteredClaims
	}

	form := web.GetForm(ctx).(*forms.IntrospectTokenForm)
	token, err := oauth2.ParseToken(form.Token, oauth2.DefaultSigningKey)
	if err == nil {
		grant, err := auth.GetOAuth2GrantByID(ctx, token.GrantID)
		if err == nil && grant != nil {
			app, err := auth.GetOAuth2ApplicationByID(ctx, grant.ApplicationID)
			if err == nil && app != nil {
				response.Active = true
				response.Scope = grant.Scope
				response.Issuer = strings.TrimSuffix(setting.AppURL, "/")
				response.Audience = []string{app.ClientID}
				response.Subject = fmt.Sprint(grant.UserID)
			}
			if user, err := user_model.GetUserByID(ctx, grant.UserID); err == nil {
				response.Username = user.Name
			}
		}
	}

	ctx.JSON(http.StatusOK, response)
}

// AuthorizeOAuth manages authorize requests
func AuthorizeOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.AuthorizationForm)
	errs := binding.Errors{}
	errs = form.Validate(ctx.Req, errs)
	if len(errs) > 0 {
		var errstring strings.Builder
		for _, e := range errs {
			errstring.WriteString(e.Error() + "\n")
		}
		ctx.ServerError("AuthorizeOAuth: Validate: ", fmt.Errorf("errors occurred during validation: %s", errstring.String()))
		return
	}

	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		if auth.IsErrOauthClientIDInvalid(err) {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeUnauthorizedClient,
				ErrorDescription: "Client ID not registered",
				State:            form.State,
			}, "")
			return
		}
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}

	var user *user_model.User
	if app.UID != 0 {
		user, err = user_model.GetUserByID(ctx, app.UID)
		if err != nil {
			ctx.ServerError("GetUserByID", err)
			return
		}
	}

	if !app.ContainsRedirectURI(form.RedirectURI) {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "Unregistered Redirect URI",
			State:            form.State,
		}, "")
		return
	}

	if form.ResponseType != "code" {
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeUnsupportedResponseType,
			ErrorDescription: "Only code response type is supported.",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	// pkce support
	switch form.CodeChallengeMethod {
	case "S256", "plain":
		if err := ctx.Session.Set("CodeChallengeMethod", form.CodeChallengeMethod); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge method",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		if err := ctx.Session.Set("CodeChallenge", form.CodeChallenge); err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeServerError,
				ErrorDescription: "cannot set code challenge",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
		// Here we're just going to try to release the session early
		if err := ctx.Session.Release(); err != nil {
			// we'll tolerate errors here as they *should* get saved elsewhere
			log.Error("Unable to save changes to the session: %v", err)
		}
	case "":
		// "Authorization servers SHOULD reject authorization requests from native apps that don't use PKCE by returning an error message"
		// https://datatracker.ietf.org/doc/html/rfc8252#section-8.1
		if !app.ConfidentialClient {
			// "the authorization endpoint MUST return the authorization error response with the "error" value set to "invalid_request""
			// https://datatracker.ietf.org/doc/html/rfc7636#section-4.4.1
			handleAuthorizeError(ctx, AuthorizeError{
				ErrorCode:        ErrorCodeInvalidRequest,
				ErrorDescription: "PKCE is required for public clients",
				State:            form.State,
			}, form.RedirectURI)
			return
		}
	default:
		// "If the server supporting PKCE does not support the requested transformation, the authorization endpoint MUST return the authorization error response with "error" value set to "invalid_request"."
		// https://www.rfc-editor.org/rfc/rfc7636#section-4.4.1
		handleAuthorizeError(ctx, AuthorizeError{
			ErrorCode:        ErrorCodeInvalidRequest,
			ErrorDescription: "unsupported code challenge method",
			State:            form.State,
		}, form.RedirectURI)
		return
	}

	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		return
	}

	// Redirect if user already granted access and the application is confidential.
	// I.e. always require authorization for public clients as recommended by RFC 6749 Section 10.2
	if app.ConfidentialClient && grant != nil {
		code, err := grant.GenerateNewAuthorizationCode(ctx, form.RedirectURI, form.CodeChallenge, form.CodeChallengeMethod)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI, err)
			return
		}
		redirect, err := code.GenerateRedirectURI(form.State)
		if err != nil {
			handleServerError(ctx, form.State, form.RedirectURI, err)
			return
		}
		// Update nonce to reflect the new session
		if len(form.Nonce) > 0 {
			err := grant.SetNonce(ctx, form.Nonce)
			if err != nil {
				log.Error("Unable to update nonce: %v", err)
			}
		}
		ctx.Redirect(redirect.String())
		return
	}

	// show authorize page to grant access
	ctx.Data["Application"] = app
	ctx.Data["RedirectURI"] = form.RedirectURI
	ctx.Data["State"] = form.State
	ctx.Data["Scope"] = form.Scope
	ctx.Data["Nonce"] = form.Nonce
	if user != nil {
		ctx.Data["ApplicationCreatorLinkHTML"] = template.HTML(fmt.Sprintf(`<a href="%s">@%s</a>`, html.EscapeString(user.HomeLink()), html.EscapeString(user.Name)))
	} else {
		ctx.Data["ApplicationCreatorLinkHTML"] = template.HTML(fmt.Sprintf(`<a href="%s">%s</a>`, html.EscapeString(setting.AppSubURL+"/"), html.EscapeString(setting.AppName)))
	}
	ctx.Data["ApplicationRedirectDomainHTML"] = template.HTML("<strong>" + html.EscapeString(form.RedirectURI) + "</strong>")
	// TODO document SESSION <=> FORM
	err = ctx.Session.Set("client_id", app.ClientID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("redirect_uri", form.RedirectURI)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		log.Error(err.Error())
		return
	}
	err = ctx.Session.Set("state", form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		log.Error(err.Error())
		return
	}
	// Here we're just going to try to release the session early
	if err := ctx.Session.Release(); err != nil {
		// we'll tolerate errors here as they *should* get saved elsewhere
		log.Error("Unable to save changes to the session: %v", err)
	}
	ctx.HTML(http.StatusOK, tplGrantAccess)
}

// GrantApplicationOAuth manages the post request submitted when a user grants access to an application
func GrantApplicationOAuth(ctx *context.Context) {
	form := web.GetForm(ctx).(*forms.GrantApplicationForm)
	if ctx.Session.Get("client_id") != form.ClientID || ctx.Session.Get("state") != form.State ||
		ctx.Session.Get("redirect_uri") != form.RedirectURI {
		ctx.Error(http.StatusBadRequest)
		return
	}

	if !form.Granted {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "the request is denied",
			ErrorCode:        ErrorCodeAccessDenied,
		}, form.RedirectURI)
		return
	}

	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		ctx.ServerError("GetOAuth2ApplicationByClientID", err)
		return
	}
	grant, err := app.GetGrantByUserID(ctx, ctx.Doer.ID)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		return
	}
	if grant == nil {
		grant, err = app.CreateGrant(ctx, ctx.Doer.ID, form.Scope)
		if err != nil {
			handleAuthorizeError(ctx, AuthorizeError{
				State:            form.State,
				ErrorDescription: "cannot create grant for user",
				ErrorCode:        ErrorCodeServerError,
			}, form.RedirectURI)
			return
		}
	} else if grant.Scope != form.Scope {
		handleAuthorizeError(ctx, AuthorizeError{
			State:            form.State,
			ErrorDescription: "a grant exists with different scope",
			ErrorCode:        ErrorCodeServerError,
		}, form.RedirectURI)
		return
	}

	if len(form.Nonce) > 0 {
		err := grant.SetNonce(ctx, form.Nonce)
		if err != nil {
			log.Error("Unable to update nonce: %v", err)
		}
	}

	var codeChallenge, codeChallengeMethod string
	codeChallenge, _ = ctx.Session.Get("CodeChallenge").(string)
	codeChallengeMethod, _ = ctx.Session.Get("CodeChallengeMethod").(string)

	code, err := grant.GenerateNewAuthorizationCode(ctx, form.RedirectURI, codeChallenge, codeChallengeMethod)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		return
	}
	redirect, err := code.GenerateRedirectURI(form.State)
	if err != nil {
		handleServerError(ctx, form.State, form.RedirectURI, err)
		return
	}
	ctx.Redirect(redirect.String(), http.StatusSeeOther)
}

// OIDCWellKnown generates JSON so OIDC clients know Gitea's capabilities
func OIDCWellKnown(ctx *context.Context) {
	if !setting.OAuth2.Enabled {
		ctx.Status(http.StatusNotFound)
		return
	}

	ctx.Data["SigningAlg"] = oauth2.DefaultSigningKey.SigningMethod().Alg()
	ctx.Data["Issuer"] = strings.TrimSuffix(setting.AppURL, "/")
	ctx.JSONTemplate("user/auth/oidc_wellknown")
}

// OIDCKeys generates the JSON Web Key Set
func OIDCKeys(ctx *context.Context) {
	jwk, err := oauth2.DefaultSigningKey.ToJWK()
	if err != nil {
		log.Error("Error converting signing key to JWK: %v", err)
		ctx.Error(http.StatusInternalServerError)
		return
	}

	jwk["use"] = "sig"

	jwks := map[string][]map[string]string{
		"keys": {
			jwk,
		},
	}

	ctx.Resp.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(ctx.Resp)
	if err := enc.Encode(jwks); err != nil {
		log.Error("Failed to encode representation as json. Error: %v", err)
	}
}

// AccessTokenOAuth manages all access token requests by the client
func AccessTokenOAuth(ctx *context.Context) {
	form := *web.GetForm(ctx).(*forms.AccessTokenForm)
	// if there is no ClientID or ClientSecret in the request body, fill these fields by the Authorization header and ensure the provided field matches the Authorization header
	if form.ClientID == "" || form.ClientSecret == "" {
		authHeader := ctx.Req.Header.Get("Authorization")
		if authType, authData, ok := strings.Cut(authHeader, " "); ok && strings.EqualFold(authType, "Basic") {
			clientID, clientSecret, err := base.BasicAuthDecode(authData)
			if err != nil {
				handleAccessTokenError(ctx, AccessTokenErrorResponse{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "cannot parse basic auth header",
				})
				return
			}
			// validate that any fields present in the form match the Basic auth header
			if form.ClientID != "" && form.ClientID != clientID {
				handleAccessTokenError(ctx, AccessTokenErrorResponse{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "client_id in request body inconsistent with Authorization header",
				})
				return
			}
			form.ClientID = clientID
			if form.ClientSecret != "" && form.ClientSecret != clientSecret {
				handleAccessTokenError(ctx, AccessTokenErrorResponse{
					ErrorCode:        AccessTokenErrorCodeInvalidRequest,
					ErrorDescription: "client_secret in request body inconsistent with Authorization header",
				})
				return
			}
			form.ClientSecret = clientSecret
		}
	}

	serverKey := oauth2.DefaultSigningKey
	clientKey := serverKey
	if serverKey.IsSymmetric() {
		var err error
		clientKey, err = jwtx.CreateSigningKey(serverKey.SigningMethod().Alg(), []byte(form.ClientSecret))
		if err != nil {
			handleAccessTokenError(ctx, AccessTokenErrorResponse{
				ErrorCode:        AccessTokenErrorCodeInvalidRequest,
				ErrorDescription: "Error creating signing key",
			})
			return
		}
	}

	switch form.GrantType {
	case "refresh_token":
		handleRefreshToken(ctx, form, serverKey, clientKey)
	case "authorization_code":
		handleAuthorizationCode(ctx, form, serverKey, clientKey)
	default:
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnsupportedGrantType,
			ErrorDescription: "Only refresh_token or authorization_code grant type is supported",
		})
	}
}

func handleRefreshToken(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey jwtx.SigningKey) {
	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: %q", form.ClientID),
		})
		return
	}
	// "The authorization server MUST ... require client authentication for confidential clients"
	// https://datatracker.ietf.org/doc/html/rfc6749#section-6
	if app.ConfidentialClient && !app.ValidateClientSecret([]byte(form.ClientSecret)) {
		errorDescription := "invalid client secret"
		if form.ClientSecret == "" {
			errorDescription = "invalid empty client secret"
		}
		// "invalid_client ... Client authentication failed"
		// https://datatracker.ietf.org/doc/html/rfc6749#section-5.2
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidClient,
			ErrorDescription: errorDescription,
		})
		return
	}

	token, err := oauth2.ParseToken(form.RefreshToken, serverKey)
	if err != nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "unable to parse refresh token",
		})
		return
	}

	// Reject tokens that are not refresh tokens (e.g. access tokens submitted as refresh tokens)
	if token.Type != oauth2.TypeRefreshToken {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "token is not a refresh token",
		})
		return
	}

	// get grant before increasing counter
	grant, err := auth.GetOAuth2GrantByID(ctx, token.GrantID)
	if err != nil || grant == nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "grant does not exist",
		})
		return
	}

	// Ensure the refresh token's grant belongs to the requesting client.
	// This prevents cross-client token usage (RFC 6749 Section 10.4).
	if grant.ApplicationID != app.ID {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "refresh token was not issued to this client",
		})
		return
	}

	// check if token got already used
	if setting.OAuth2.InvalidateRefreshTokens && (grant.Counter != token.Counter || token.Counter == 0) {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "token was already used",
		})
		log.Warn("A client tried to use a refresh token for grant_id = %d was used twice!", grant.ID)
		return
	}
	accessToken, tokenErr := newAccessTokenResponse(ctx, grant, serverKey, clientKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	ctx.JSON(http.StatusOK, accessToken)
}

func handleAuthorizationCode(ctx *context.Context, form forms.AccessTokenForm, serverKey, clientKey jwtx.SigningKey) {
	app, err := auth.GetOAuth2ApplicationByClientID(ctx, form.ClientID)
	if err != nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidClient,
			ErrorDescription: fmt.Sprintf("cannot load client with client id: '%s'", form.ClientID),
		})
		return
	}
	if app.ConfidentialClient && !app.ValidateClientSecret([]byte(form.ClientSecret)) {
		errorDescription := "invalid client secret"
		if form.ClientSecret == "" {
			errorDescription = "invalid empty client secret"
		}
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: errorDescription,
		})
		return
	}
	if form.RedirectURI != "" && !app.ContainsRedirectURI(form.RedirectURI) {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "unexpected redirect URI",
		})
		return
	}
	authorizationCode, err := auth.GetOAuth2AuthorizationByCode(ctx, form.Code)
	if err != nil || authorizationCode == nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "client is not authorized",
		})
		return
	}
	// check if code verifier authorizes the client, PKCE support
	if !authorizationCode.ValidateCodeChallenge(form.CodeVerifier) {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "failed PKCE code challenge",
		})
		return
	}
	// Per RFC 6749 §4.1.3, if redirect_uri was included in the authorization request,
	// it MUST be identical to the value included in the token request.
	if authorizationCode.RedirectURI != "" && form.RedirectURI != authorizationCode.RedirectURI {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeUnauthorizedClient,
			ErrorDescription: "redirect_uri does not match the authorization request",
		})
		return
	}
	// check if granted for this application
	if authorizationCode.Grant.ApplicationID != app.ID {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidGrant,
			ErrorDescription: "invalid grant",
		})
		return
	}
	// remove token from database to deny duplicate usage
	if err := authorizationCode.Invalidate(ctx); err != nil {
		handleAccessTokenError(ctx, AccessTokenErrorResponse{
			ErrorCode:        AccessTokenErrorCodeInvalidRequest,
			ErrorDescription: "cannot proceed your request",
		})
		return
	}
	resp, tokenErr := newAccessTokenResponse(ctx, authorizationCode.Grant, serverKey, clientKey)
	if tokenErr != nil {
		handleAccessTokenError(ctx, *tokenErr)
		return
	}
	// send successful response
	ctx.JSON(http.StatusOK, resp)
}

func handleAccessTokenError(ctx *context.Context, acErr AccessTokenErrorResponse) {
	ctx.JSON(http.StatusBadRequest, acErr)
}

func handleServerError(ctx *context.Context, state, redirectURI string, err error) {
	log.Error("OAuth server error: %v", err)
	handleAuthorizeError(ctx, AuthorizeError{
		ErrorCode:        ErrorCodeServerError,
		ErrorDescription: "A server error occurred",
		State:            state,
	}, redirectURI)
}

func handleAuthorizeError(ctx *context.Context, authErr AuthorizeError, redirectURI string) {
	if redirectURI == "" {
		log.Warn("Authorization failed: %v", authErr.ErrorDescription)
		ctx.Data["Error"] = authErr
		ctx.HTML(http.StatusBadRequest, tplGrantError)
		return
	}
	redirect, err := url.Parse(redirectURI)
	if err != nil {
		ctx.ServerError("url.Parse", err)
		return
	}
	q := redirect.Query()
	q.Set("error", string(authErr.ErrorCode))
	q.Set("error_description", authErr.ErrorDescription)
	q.Set("state", authErr.State)
	redirect.RawQuery = q.Encode()
	ctx.Redirect(redirect.String(), http.StatusSeeOther)
}

// SignInOAuth handles the OAuth2 login buttons
func SignInOAuth(ctx *context.Context) {
	provider := ctx.Params(":provider")

	authSource, err := auth.GetActiveOAuth2SourceByName(ctx, provider)
	if err != nil {
		ctx.ServerError("SignIn", err)
		return
	}

	redirectTo := ctx.FormString("redirect_to")
	if len(redirectTo) > 0 {
		middleware.SetRedirectToCookie(ctx.Resp, redirectTo)
	}

	// Overwrite to avoid leaking the value from a prior attempt.
	promptParam := ctx.FormString("prompt")
	if err := ctx.Session.Set("oauth_signin_silent", promptParam == "none"); err != nil {
		ctx.ServerError("Session.Set", err)
		return
	}
	if err := ctx.Session.Release(); err != nil {
		ctx.ServerError("Session.Release", err)
		return
	}

	// try to do a direct callback flow, so we don't authenticate the user again but use the valid accesstoken to get the user
	user, gothUser, err := oAuth2UserLoginCallback(ctx, authSource, ctx.Req, ctx.Resp)
	if err == nil && user != nil {
		// we got the user without going through the whole OAuth2 authentication flow again
		handleOAuth2SignIn(ctx, authSource, user, gothUser)
		return
	}

	codeChallenge, err := generateCodeChallenge(ctx, provider)
	if err != nil {
		ctx.ServerError("SignIn", fmt.Errorf("could not generate code_challenge: %w", err))
		return
	}

	if err = authSource.Cfg.(*oauth2.Source).Callout(ctx.Req, ctx.Resp, codeChallenge, promptParam); err != nil {
		if strings.Contains(err.Error(), "no provider for ") {
			if err = oauth2.ResetOAuth2(ctx); err != nil {
				ctx.ServerError("SignIn", err)
				return
			}
			if err = authSource.Cfg.(*oauth2.Source).Callout(ctx.Req, ctx.Resp, codeChallenge, promptParam); err != nil {
				ctx.ServerError("SignIn", err)
			}
			return
		}
		ctx.ServerError("SignIn", err)
	}
	// redirect is done in oauth2.Auth
}

// SignInOAuthCallback handles the callback from the given provider
func SignInOAuthCallback(ctx *context.Context) {
	provider := ctx.Params(":provider")

	// If the IdP refused our prompt=none silent re-auth, retry interactively rather than surfacing the error.
	if isOIDCSilentAuthFailure(ctx.Req.FormValue("error")) {
		if silent, _ := ctx.Session.Get("oauth_signin_silent").(bool); silent {
			if err := ctx.Session.Delete("oauth_signin_silent"); err != nil {
				ctx.ServerError("Session.Delete", err)
				return
			}
			if err := ctx.Session.Release(); err != nil {
				ctx.ServerError("Session.Release", err)
				return
			}
			ctx.Redirect(fmt.Sprintf("%s/user/oauth2/%s", setting.AppSubURL, url.PathEscape(provider)))
			return
		}
	}

	if ctx.Req.FormValue("error") != "" {
		var errorKeyValues []string
		for k, vv := range ctx.Req.Form {
			for _, v := range vv {
				errorKeyValues = append(errorKeyValues, fmt.Sprintf("%s = %s", html.EscapeString(k), html.EscapeString(v)))
			}
		}
		sort.Strings(errorKeyValues)
		ctx.Flash.Error(strings.Join(errorKeyValues, "<br>"), true)
	}

	// first look if the provider is still active
	authSource, err := auth.GetActiveOAuth2SourceByName(ctx, provider)
	if err != nil {
		ctx.ServerError("SignIn", err)
		return
	}

	if authSource == nil {
		ctx.ServerError("SignIn", errors.New("no valid provider found, check configured callback url in provider"))
		return
	}

	u, gothUser, err := oAuth2UserLoginCallback(ctx, authSource, ctx.Req, ctx.Resp)

	log.Trace("OAuth2 Provider %s returned gothUser: UserID=%q, Email=%q, NickName=%q, Name=%q, FirstName=%q, LastName=%q, AvatarURL=%q",
		authSource.Name, gothUser.UserID, gothUser.Email, gothUser.NickName, gothUser.Name, gothUser.FirstName, gothUser.LastName, gothUser.AvatarURL)
	if gothUser.RawData != nil {
		log.Trace("OAuth2 Provider %s RawData: %+v", authSource.Name, gothUser.RawData)
	}
	if gothUser.IDToken != "" {
		log.Trace("OAuth2 Provider %s IDToken (decode at jwt.ms): %s", authSource.Name, gothUser.IDToken)
	}

	if err != nil {
		if user_model.IsErrUserProhibitLogin(err) {
			uplerr := err.(user_model.ErrUserProhibitLogin)
			log.Info("Failed authentication attempt for %s from %s: %v", uplerr.Name, ctx.RemoteAddr(), err)
			ctx.Data["Title"] = ctx.Tr("auth.prohibit_login")
			ctx.HTML(http.StatusOK, "user/auth/prohibit_login")
			return
		}
		if callbackErr, ok := err.(errCallback); ok {
			log.Info("Failed OAuth callback: (%v) %v", callbackErr.Code, callbackErr.Description)
			switch callbackErr.Code {
			case "access_denied":
				ctx.Flash.Error(ctx.Tr("auth.oauth.signin.error.access_denied"))
			case "temporarily_unavailable":
				ctx.Flash.Error(ctx.Tr("auth.oauth.signin.error.temporarily_unavailable"))
			default:
				ctx.Flash.Error(ctx.Tr("auth.oauth.signin.error"))
			}
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}
		if err, ok := err.(*go_oauth2.RetrieveError); ok {
			ctx.Flash.Error("OAuth2 RetrieveError: " + err.Error())
			ctx.Redirect(setting.AppSubURL + "/user/login")
			return
		}
		ctx.ServerError("UserSignIn", err)
		return
	}

	if u == nil {
		if ctx.Doer != nil {
			// attach user to already logged in user
			err = externalaccount.LinkAccountToUser(ctx, ctx.Doer, gothUser)
			if err != nil {
				ctx.ServerError("UserLinkAccount", err)
				return
			}

			ctx.Redirect(setting.AppSubURL + "/user/settings/security")
			return
		} else if !setting.Service.AllowOnlyInternalRegistration && setting.OAuth2Client.EnableAutoRegistration {
			// create new user with details from oauth2 provider
			if gothUser.UserID == "" {
				log.Error("OAuth2 Provider %s returned empty or missing field: UserID", authSource.Name)
				if authSource.IsOAuth2() && authSource.Cfg.(*oauth2.Source).Provider == "openidConnect" {
					log.Error("You may need to change the 'OPENID_CONNECT_SCOPES' setting to request all required fields")
				}
				err = fmt.Errorf("OAuth2 Provider %s returned empty or missing field: UserID", authSource.Name)
				ctx.ServerError("CreateUser", err)
				return
			}
			var missingFields []string
			if gothUser.Email == "" {
				missingFields = append(missingFields, "email")
			}
			if setting.OAuth2Client.Username == setting.OAuth2UsernameNickname && gothUser.NickName == "" {
				missingFields = append(missingFields, "nickname")
			} else if setting.OAuth2Client.Username == setting.OAuth2UsernamePreferredUsername && (gothUser.RawData["preferred_username"] == nil || gothUser.RawData["preferred_username"].(string) == "") {
				missingFields = append(missingFields, "preferred_username")
			}
			if len(missingFields) > 0 {
				// we don't have enough information to create an account automatically,
				// so we prompt the user for the remaining bits
				log.Trace("OAuth2 Provider %s returned empty or missing fields: %s, prompting the user for them", authSource.Name, missingFields)
				showLinkingLogin(ctx, gothUser)
				return
			}
			uname, err := getUserName(&gothUser)
			if err != nil {
				ctx.ServerError("UserSignIn", err)
				return
			}
			u = &user_model.User{
				Name:        uname,
				FullName:    gothUser.Name,
				Email:       gothUser.Email,
				LoginType:   auth.OAuth2,
				LoginSource: authSource.ID,
				LoginName:   gothUser.UserID,
			}

			overwriteDefault := &user_model.CreateUserOverwriteOptions{
				IsActive: optional.Some(!setting.OAuth2Client.RegisterEmailConfirm && !setting.Service.RegisterManualConfirm),
			}

			source := authSource.Cfg.(*oauth2.Source)

			isAdmin, isRestricted := getUserAdminAndRestrictedFromGroupClaims(source, &gothUser)
			u.IsAdmin = isAdmin.ValueOrDefault(false)
			u.IsRestricted = isRestricted.ValueOrDefault(setting.Service.DefaultUserIsRestricted)

			if !createAndHandleCreatedUser(ctx, base.TplName(""), nil, u, overwriteDefault, &gothUser, setting.OAuth2Client.AccountLinking != setting.OAuth2AccountLinkingDisabled) {
				// error already handled
				return
			}

			if err := syncGroupsToTeams(ctx, authSource, &gothUser, u); err != nil {
				ctx.ServerError("SyncGroupsToTeams", err)
				return
			}

			if err := syncGroupsToQuotaGroups(ctx, source, &gothUser, u); err != nil {
				ctx.ServerError("SyncGroupsToQuotaGroups", err)
				return
			}
		} else {
			// no existing user is found, request attach or new account
			showLinkingLogin(ctx, gothUser)
			return
		}
	}

	handleOAuth2SignIn(ctx, authSource, u, gothUser)
}

func claimValueToStringSet(claimValue any) container.Set[string] {
	var groups []string

	switch rawGroup := claimValue.(type) {
	case []string:
		groups = rawGroup
	case []any:
		for _, group := range rawGroup {
			groups = append(groups, fmt.Sprintf("%s", group))
		}
	default:
		str := fmt.Sprintf("%s", rawGroup)
		groups = strings.Split(str, ",")
	}
	return container.SetOf(groups...)
}

func syncGroupsToTeams(ctx *context.Context, authSource *auth.Source, gothUser *goth.User, u *user_model.User) error {
	source := authSource.Cfg.(*oauth2.Source)
	if source.GroupTeamMap != "" || source.GroupTeamMapRemoval ||
		source.DynGroupMaps != "" || source.DynGroupMapsRemoval {
		groupTeamMapping, err := auth_module.UnmarshalGroupTeamMapping(source.GroupTeamMap)
		if err != nil {
			return err
		}

		dynGroupMappings, err := auth_module.UnmarshalDynGroupMappings(source.DynGroupMaps)
		if err != nil {
			return err
		}
		dynGroupMaps := source_service.GetDynGroupMaps(authSource.ID, dynGroupMappings)

		groups := getClaimedGroups(source, gothUser)

		if err := source_service.SyncGroupsToTeams(ctx,
			u, groups, groupTeamMapping, source.GroupTeamMapRemoval,
			dynGroupMaps, source.DynGroupMapsRemoval,
		); err != nil {
			return err
		}
	}

	return nil
}

func syncGroupsToQuotaGroups(ctx *context.Context, source *oauth2.Source, gothUser *goth.User, u *user_model.User) error {
	if source.QuotaGroupMap != "" || source.QuotaGroupMapRemoval {
		quotaGroupMapping, err := auth_module.UnmarshalQuotaGroupMapping(source.QuotaGroupMap)
		if err != nil {
			return err
		}

		groups := getClaimedQuotaGroups(source, gothUser)

		if err := source_service.SyncGroupsToQuotaGroups(ctx, u, groups, quotaGroupMapping, source.QuotaGroupMapRemoval); err != nil {
			return err
		}
	}

	return nil
}

func getClaimedGroups(source *oauth2.Source, gothUser *goth.User) container.Set[string] {
	groupClaims, has := gothUser.RawData[source.GroupClaimName]
	if !has {
		return nil
	}

	return claimValueToStringSet(groupClaims)
}

func getClaimedQuotaGroups(source *oauth2.Source, gothUser *goth.User) container.Set[string] {
	groupClaims, has := gothUser.RawData[source.QuotaGroupClaimName]
	if !has {
		return nil
	}

	return claimValueToStringSet(groupClaims)
}

func getUserAdminAndRestrictedFromGroupClaims(source *oauth2.Source, gothUser *goth.User) (isAdmin, isRestricted optional.Option[bool]) {
	groups := getClaimedGroups(source, gothUser)

	if source.AdminGroup != "" {
		isAdmin = optional.Some(groups.Contains(source.AdminGroup))
	}
	if source.RestrictedGroup != "" {
		isRestricted = optional.Some(groups.Contains(source.RestrictedGroup))
	}

	return isAdmin, isRestricted
}

func showLinkingLogin(ctx *context.Context, gothUser goth.User) {
	if err := updateSession(ctx, nil, map[string]any{
		"linkAccountGothUser": gothUser,
	}); err != nil {
		ctx.ServerError("updateSession", err)
		return
	}
	ctx.Redirect(setting.AppSubURL + "/user/link_account")
}

func updateAvatarIfNeed(ctx *context.Context, url string, u *user_model.User) {
	if setting.OAuth2Client.UpdateAvatar && len(url) > 0 {
		resp, err := http.Get(url)
		if err == nil {
			defer func() {
				_ = resp.Body.Close()
			}()
		}
		// ignore any error
		if err == nil && resp.StatusCode == http.StatusOK {
			data, err := io.ReadAll(io.LimitReader(resp.Body, setting.Avatar.MaxFileSize+1))
			if err == nil && int64(len(data)) <= setting.Avatar.MaxFileSize {
				_ = user_service.UploadAvatar(ctx, u, data)
			}
		}
	}
}

func getSSHKeys(source *oauth2.Source, gothUser *goth.User) ([]string, error) {
	key := source.AttributeSSHPublicKey
	value, exists := gothUser.RawData[key]
	if !exists {
		return []string{}, nil
	}

	rawSlice, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("unexpected type for SSH public key, expected []interface{} but got %T", value)
	}

	sshKeys := make([]string, 0, len(rawSlice))
	for i, v := range rawSlice {
		str, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("unexpected element type at index %d in SSH public key array, expected string but got %T", i, v)
		}
		sshKeys = append(sshKeys, str)
	}

	return sshKeys, nil
}

func updateSSHPubIfNeed(
	ctx *context.Context,
	authSource *auth.Source,
	fetchedUser *goth.User,
	user *user_model.User,
) error {
	oauth2Source := authSource.Cfg.(*oauth2.Source)

	if oauth2Source.ProvidesSSHKeys() {
		sshKeys, err := getSSHKeys(oauth2Source, fetchedUser)
		if err != nil {
			return err
		}

		if asymkey_model.SynchronizePublicKeys(ctx, user, authSource, sshKeys) {
			err = asymkey_model.RewriteAllPublicKeys(ctx)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func handleOAuth2SignIn(ctx *context.Context, source *auth.Source, u *user_model.User, gothUser goth.User) {
	updateAvatarIfNeed(ctx, gothUser.AvatarURL, u)
	err := updateSSHPubIfNeed(ctx, source, &gothUser, u)
	if err != nil {
		ctx.ServerError("updateSSHPubIfNeed", err)
		return
	}

	needs2FA := false
	if !source.Cfg.(*oauth2.Source).SkipLocalTwoFA {
		needs2FA, err = auth.HasTwoFactorByUID(ctx, u.ID)
		if err != nil {
			ctx.ServerError("UserSignIn", err)
			return
		}
	}

	oauth2Source := source.Cfg.(*oauth2.Source)
	groupTeamMapping, err := auth_module.UnmarshalGroupTeamMapping(oauth2Source.GroupTeamMap)
	if err != nil {
		ctx.ServerError("UnmarshalGroupTeamMapping", err)
		return
	}
	quotaGroupMapping, err := auth_module.UnmarshalQuotaGroupMapping(oauth2Source.QuotaGroupMap)
	if err != nil {
		ctx.ServerError("UnmarshalQuotaGroupMapping", err)
		return
	}

	dynGroupMappings, err := auth_module.UnmarshalDynGroupMappings(oauth2Source.DynGroupMaps)
	if err != nil {
		ctx.ServerError("UnmarshalDynGroupMappings", err)
		return
	}
	dynGroupMaps := source_service.NewDynGroupMaps(dynGroupMappings)

	groups := getClaimedGroups(oauth2Source, &gothUser)
	quotaGroups := getClaimedQuotaGroups(oauth2Source, &gothUser)

	// If this user is enrolled in 2FA and this source doesn't override it,
	// we can't sign the user in just yet. Instead, redirect them to the 2FA authentication page.
	if !needs2FA {
		if err := ctx.SetSSOLTACookie(u, source.ID); err != nil {
			ctx.ServerError("SetSSOLTACookie", err)
			return
		}

		if err := updateSession(ctx,
			[]string{"oauth_signin_silent"},
			map[string]any{
				"uid": u.ID,
			}); err != nil {
			ctx.ServerError("updateSession", err)
			return
		}

		opts := &user_service.UpdateOptions{
			SetLastLogin: true,
		}
		opts.IsAdmin, opts.IsRestricted = getUserAdminAndRestrictedFromGroupClaims(oauth2Source, &gothUser)
		if err := user_service.UpdateUser(ctx, u, opts); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}

		if oauth2Source.GroupTeamMap != "" || oauth2Source.GroupTeamMapRemoval ||
			oauth2Source.DynGroupMaps != "" || oauth2Source.DynGroupMapsRemoval {
			if err := source_service.SyncGroupsToTeams(ctx,
				u, groups, groupTeamMapping, oauth2Source.GroupTeamMapRemoval,
				dynGroupMaps, oauth2Source.DynGroupMapsRemoval,
			); err != nil {
				ctx.ServerError("SyncGroupsToTeams", err)
				return
			}
		}

		if oauth2Source.QuotaGroupMap != "" || oauth2Source.QuotaGroupMapRemoval {
			if err := source_service.SyncGroupsToQuotaGroups(ctx, u, quotaGroups, quotaGroupMapping, oauth2Source.QuotaGroupMapRemoval); err != nil {
				ctx.ServerError("SyncGroupsToQuotaGroups", err)
				return
			}
		}

		// update external user information
		if err := externalaccount.UpdateExternalUser(ctx, u, gothUser); err != nil {
			if !errors.Is(err, util.ErrNotExist) {
				log.Error("UpdateExternalUser failed: %v", err)
			}
		}

		if err := resetLocale(ctx, u); err != nil {
			ctx.ServerError("resetLocale", err)
			return
		}

		if redirectTo := ctx.GetSiteCookie("redirect_to"); len(redirectTo) > 0 {
			middleware.DeleteRedirectToCookie(ctx.Resp)
			ctx.RedirectToFirst(redirectTo)
			return
		}

		ctx.Redirect(setting.AppSubURL + "/")
		return
	}

	opts := &user_service.UpdateOptions{}
	opts.IsAdmin, opts.IsRestricted = getUserAdminAndRestrictedFromGroupClaims(oauth2Source, &gothUser)
	if opts.IsAdmin.Has() || opts.IsRestricted.Has() {
		if err := user_service.UpdateUser(ctx, u, opts); err != nil {
			ctx.ServerError("UpdateUser", err)
			return
		}
	}

	if oauth2Source.GroupTeamMap != "" || oauth2Source.GroupTeamMapRemoval ||
		oauth2Source.DynGroupMaps != "" || oauth2Source.DynGroupMapsRemoval {
		if err := source_service.SyncGroupsToTeams(ctx,
			u, groups, groupTeamMapping, oauth2Source.GroupTeamMapRemoval,
			dynGroupMaps, oauth2Source.DynGroupMapsRemoval,
		); err != nil {
			ctx.ServerError("SyncGroupsToTeams", err)
			return
		}
	}

	if oauth2Source.QuotaGroupMap != "" || oauth2Source.QuotaGroupMapRemoval {
		if err := source_service.SyncGroupsToQuotaGroups(ctx, u, quotaGroups, quotaGroupMapping, oauth2Source.QuotaGroupMapRemoval); err != nil {
			ctx.ServerError("SyncGroupsToQuotaGroups", err)
			return
		}
	}

	if err := updateSession(ctx,
		[]string{"oauth_signin_silent"},
		map[string]any{
			"twofaUid":            u.ID,
			"twofaRemember":       true, // OAuth implies remember
			"twofaSSOLTA":         true, // honored by handleSignInFull to issue the SSO variant
			"twofaSSOLTASourceID": source.ID,
		}); err != nil {
		ctx.ServerError("updateSession", err)
		return
	}

	// If WebAuthn is enrolled -> Redirect to WebAuthn instead
	regs, err := auth.GetWebAuthnCredentialsByUID(ctx, u.ID)
	if err == nil && len(regs) > 0 {
		ctx.Redirect(setting.AppSubURL + "/user/webauthn")
		return
	}

	ctx.Redirect(setting.AppSubURL + "/user/two_factor")
}

// generateCodeChallenge stores a code verifier in the session and returns a S256 code challenge for PKCE
func generateCodeChallenge(ctx *context.Context, provider string) (codeChallenge string, err error) {
	// the `code_verifier` is only forwarded by specific providers
	// https://codeberg.org/forgejo/forgejo/issues/4033
	p, ok := goth.GetProviders()[provider]
	if !ok {
		return "", nil
	}
	switch p.(type) {
	default:
		return "", nil
	case *openidConnect.Provider, *fitbit.Provider, *zoom.Provider:
		// those providers forward the `code_verifier`
		// a code_challenge can be generated
		break
	}

	codeVerifier := util.CryptoRandomString(util.RandomStringHigh)
	if err = ctx.Session.Set("CodeVerifier", codeVerifier); err != nil {
		return "", err
	}
	return encodeCodeChallenge(codeVerifier)
}

func encodeCodeChallenge(codeVerifier string) (string, error) {
	hasher := sha256.New()
	_, err := io.WriteString(hasher, codeVerifier)
	codeChallenge := base64.RawURLEncoding.EncodeToString(hasher.Sum(nil))
	return codeChallenge, err
}

// >>> @@@ STACKIT CODE @@@
// User Story 44186

func createLocalUser(ctx go_context.Context, source *auth.Source, gothUser goth.User, userName string) error {
	password, err := pwd.Generate(DEFAULTPWDLENGTH)
	if err != nil {
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	// Add some fields reqired for a new remote user emulating thefunc MaybePromoteRemoteUser
	// The parameters are set to allow the assoc goth user to user match the oauth user
	u := user_model.User{
		FullName:           strings.ReplaceAll(strings.TrimSpace(gothUser.Name), " ", "_"), // The full name cannot contain spaces
		Email:              gothUser.Email,
		LoginName:          gothUser.UserID,
		IsAdmin:            false,
		IsActive:           true,
		MustChangePassword: false,
		LoginType:          auth.OAuth2,
		Type:               user_model.UserTypeIndividual,
		Name:               userName, // The login name is the email without the domain, is correct syntax for git users
		LoginSource:        source.ID,
		Language:           "en-US", // Default language for all new accounts
	}
	err = u.SetPassword(password)
	if err != nil {
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	log.Debug("Going to generate avatar for user %s", gothUser.Email)
	if err := user_model.GenerateRandomAvatar(ctx, &u); err != nil {
		log.Error("Failed to generate avatar for user %s: %v", gothUser.Email, err)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}
	log.Debug("Generated avatar for user %s", gothUser.Email)

	// Some data arrangements
	u.LowerName = strings.ToLower(u.FullName)

	// Create the user in the local database of users.
	err = user_model.CreateUser(ctx, &u)
	if err != nil {
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}
	return nil
}

// For a given Oauth2 user, goes to the STACKIT apis and check if one of its organizations is the one associated with the instance (orgID)
func checkUserInOrganization(ctx go_context.Context, orgID string, gothUser goth.User) error {
	log.Debug("checkUserInOrganization: checking user=%s against orgId=%s", gothUser.Email, orgID)
	if orgID == "" {
		log.Debug("checkUserInOrganization: orgId is empty, denying login for user=%s", gothUser.Email)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	bearerToken := gothUser.AccessToken

	clientrm, err := resourcemanager.NewAPIClient(config.WithToken(bearerToken))
	if err != nil {
		log.Debug("checkUserInOrganization: failed to create resourcemanager client: %v", err)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	orgRequest := clientrm.GetOrganization(ctx, orgID)
	orgInfo, err := orgRequest.Execute()
	if err != nil {
		log.Debug("checkUserInOrganization: failed to get organization info for orgId=%s: %v", orgID, err)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	// The authorization api works with containers id's not with id's
	orgName := *orgInfo.ContainerId
	log.Debug("checkUserInOrganization: resolved orgName=%s for orgId=%s", orgName, orgID)

	clientauth, err := authorization.NewAPIClient(config.WithToken(bearerToken))
	if err != nil {
		log.Debug("checkUserInOrganization: failed to create authorization client: %v", err)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	// The user with email x can only read the permission of the user x or its subordinates
	getMembershipsResp, err := clientauth.ListUserMemberships(ctx, gothUser.Email).Execute()
	if err != nil {
		log.Debug("checkUserInOrganization: failed to list memberships for user=%s: %v", gothUser.Email, err)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}
	log.Debug("checkUserInOrganization: got %d memberships for user=%s", len(*getMembershipsResp.Items), gothUser.Email)

	bOrgFound := false
	for _, memberships := range *getMembershipsResp.Items {
		if *memberships.ResourceId == orgName && *memberships.ResourceType == "organization" {
			log.Debug("checkUserInOrganization: found matching organization membership for user=%s orgName=%s", gothUser.Email, orgName)
			bOrgFound = true
			break
		}
	}

	if !bOrgFound {
		log.Debug("checkUserInOrganization: user=%s is not a member of orgName=%s, denying login", gothUser.Email, orgName)
		return user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	log.Debug("checkUserInOrganization: user=%s successfully verified in organization=%s", gothUser.Email, orgName)
	return nil
}

func checkIsAdmin(ctx go_context.Context, gothUser goth.User) (bool, error) {
	// The token used is the one obtained from the user
	bearerToken := gothUser.AccessToken

	clmem, err := authorization.NewAPIClient(config.WithToken(bearerToken))
	if err != nil {
		return false, user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	// Get the pairs permission / user for the admin permission
	// permissions := strings.Split(setting.StackitGit.AdminPermissions, ",")
	// Defensive programming: only allow one and only one admin permission

	if setting.StackitGit.AdminPermissions == "" {
		// If no admin permissions are set in the configuration, return false and continue with the login
		return false, nil
	}

	permissions := []string{setting.StackitGit.AdminPermissions}
	requ := clmem.ListUserPermissions(ctx, gothUser.Email).Permissions(permissions).ResourceType("system")
	perm, err := requ.Execute()
	if err != nil {
		// In this case we can have a typo in the configuration so just return no admin and continue with the login
		return false, nil
	}

	if len(*perm.Items) > 0 {
		// The user has at least one admin permission
		return true, nil
	}

	return false, nil
}

// Check if the current oauth2 user is in the current organization
// The TRANSPARENTORGID associated to this instance means = any user can login if it belongs to the Schwarz IDP
// The function checks if the user is an admin
// If the user is allowed to login and the local users doesn't exists then create it as local user
func oauth2CheckOrganization(ctx go_context.Context, _ *auth.Source, gothUser goth.User, orgID string) (bool, error) {
	isAdmin, err := checkIsAdmin(ctx, gothUser)
	if err != nil {
		return false, err // The correct error is already returned by createLocalUser
	}

	// If the organization is the transparent organization then the check finish here and just countinue creating the local user if necessary
	if orgID != setting.TRANSPARENTORGID && !isAdmin {
		// Check if the user is in the organization
		if err := checkUserInOrganization(ctx, orgID, gothUser); err != nil {
			return false, err // The correct error is already returned by checkUserInOrganization
		}
	}

	// The oauth user belongs to the organization or is TRANSPARENTORGID , now check if the user is in the list of local users
	return isAdmin, nil
}

// <<< @@@ STACKIT CODE @@@

// Try to get the organization ID from the project, otherwise use the default organization
func getOrganizationID(ctx go_context.Context, gothUser goth.User) (string, error) {
	// Try to get the organization ID from the project

	bearerToken := gothUser.AccessToken

	client, err := resourcemanager.NewAPIClient(config.WithToken(bearerToken))
	if err != nil {
		return setting.StackitGit.OrganizationID, fmt.Errorf("failed to create the api client: %w", err)
	}

	projectID := setting.StackitGit.ProjectID
	if projectID == "" {
		// No project ID provided, use the default organization
		return setting.StackitGit.OrganizationID, nil
	}
	projectRequest := client.GetProject(ctx, projectID)
	projectInfo, err := projectRequest.Execute()
	if err != nil {
		return setting.StackitGit.OrganizationID, fmt.Errorf("failed to get the project info from the project %s : %w", projectID, err)
	}

	// Some basic checks of parent id/type
	if projectInfo.Parent == nil {
		return setting.StackitGit.OrganizationID, fmt.Errorf("failed to get the project info from the project %s : %w", projectID, err)
	}

	if *projectInfo.Parent.Type != "organization" {
		return getParentOrgID(gothUser)
	}

	orgID := *projectInfo.Parent.Id
	if orgID != "" {
		return orgID, nil
	}

	// If not found, use the default organization
	return setting.StackitGit.OrganizationID, nil
}

// <<< @@@ STACKIT CODE @@@
// jira task STACKITGIT-996

const (
	TypeOrg     = "ORGANIZATION"
	TypeFolder  = "FOLDER"
	TypeProject = "PROJECT"
	MaxDepth    = 6
)

type parentInfo struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}
type projectInfo struct {
	Parent parentInfo `json:"parent"`
}

type folderInfo struct {
	Parent parentInfo `json:"parent"`
}

func GetSDKParentInfo(token, id, itemType string) (string, string, error) {
	log.Debug("GetSDKParentInfo: resolving parent for id=%s type=%s", id, itemType)
	// This is the part that was causing the credential file error.
	// In production, it uses the SDK. In tests, we mock this whole method.
	client, err := resourcemanager.NewAPIClient(config.WithToken(token))
	if err != nil {
		log.Debug("GetSDKParentInfo: failed to create api client: %v", err)
		return "", "", fmt.Errorf("failed to create api client: %w", err)
	}

	cfg := client.GetConfig()
	url := cfg.Servers[0].URL

	var finalURL string
	switch itemType {
	case TypeProject:
		finalURL = fmt.Sprintf("%s/v2/projects/%s", url, id)
	case TypeFolder:
		finalURL = fmt.Sprintf("%s/v2/folders/%s", url, id)
	default:
		log.Debug("GetSDKParentInfo: unknown item type %s", itemType)
		return "", "", fmt.Errorf("unknown item type %s", itemType)
	}
	log.Debug("GetSDKParentInfo: requesting url=%s", finalURL)

	body, err := HTTPGet(finalURL, map[string]string{"Authorization": "Bearer " + token})
	if err != nil {
		log.Debug("GetSDKParentInfo: HTTP GET failed: %v", err)
		return "", "", err
	}
	log.Debug("GetSDKParentInfo: received %d bytes", len(body))

	if itemType == TypeProject {
		var p projectInfo
		if err := json.Unmarshal(body, &p); err != nil {
			log.Debug("GetSDKParentInfo: failed to unmarshal project response: %v", err)
			return "", "", err
		}
		log.Debug("GetSDKParentInfo: project parent id=%s type=%s", p.Parent.ID, p.Parent.Type)
		return p.Parent.ID, p.Parent.Type, nil
	}

	var f folderInfo
	if err := json.Unmarshal(body, &f); err != nil {
		log.Debug("GetSDKParentInfo: failed to unmarshal folder response: %v", err)
		return "", "", err
	}
	log.Debug("GetSDKParentInfo: folder parent id=%s type=%s", f.Parent.ID, f.Parent.Type)
	return f.Parent.ID, f.Parent.Type, nil
}

func HTTPGet(finalURL string, headers map[string]string) ([]byte, error) {
	ctx, cancel := go_context.WithTimeout(go_context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", finalURL, nil)
	if err != nil {
		return nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func GetOrganizationID(token, itemID string) (string, error) {
	itemType := TypeProject
	log.Debug("GetOrganizationID: starting traversal from %s (type=%s)", itemID, itemType)

	for depth := range MaxDepth {
		log.Debug("GetOrganizationID: depth=%d, querying parent of %s (type=%s)", depth, itemID, itemType)
		parentID, parentType, err := GetSDKParentInfo(token, itemID, itemType)
		if err != nil {
			log.Debug("GetOrganizationID: error getting parent info at depth=%d: %v", depth, err)
			return "", err
		}
		log.Debug("GetOrganizationID: depth=%d, got parent id=%s type=%s", depth, parentID, parentType)
		if parentID == "" {
			return "", fmt.Errorf("no parent returned")
		}
		if parentType == TypeOrg {
			if _, err := uuid.Parse(parentID); err != nil {
				log.Debug("GetOrganizationID: invalid org id format: %s", parentID)
				return "", fmt.Errorf("invalid org id format")
			}
			log.Debug("GetOrganizationID: found organization id=%s at depth=%d", parentID, depth)
			return parentID, nil
		}
		itemID = parentID
		itemType = parentType
	}
	log.Debug("GetOrganizationID: organization not found after %d levels", MaxDepth)
	return "", fmt.Errorf("organization not found within max depth")
}

func getParentOrgID(gothUser goth.User) (string, error) {
	bearerToken := gothUser.AccessToken

	projectID := setting.StackitGit.ProjectID
	log.Debug("getOrganizationId: projectID=%s defaultOrgID=%s", projectID, setting.StackitGit.OrganizationID)
	if projectID == "" {
		log.Debug("getOrganizationId: no project ID configured, using default organization")
		return setting.StackitGit.OrganizationID, nil
	}

	orgID, err := GetOrganizationID(bearerToken, projectID)
	if err != nil {
		log.Debug("getOrganizationId: GetOrganizationID failed: %v, falling back to default", err)
		return setting.StackitGit.OrganizationID, fmt.Errorf("failed to create the api client: %w", err)
	}

	if orgID != "" {
		log.Debug("getOrganizationId: resolved orgID=%s from project", orgID)
		return orgID, nil
	}

	log.Debug("getOrganizationId: no org ID resolved, using default organization")
	return setting.StackitGit.OrganizationID, nil
}

// end of jira task STACKITGIT-996

// OAuth2UserLoginCallback attempts to handle the callback from the OAuth2 provider and if successful
// login the user
func oAuth2UserLoginCallback(ctx *context.Context, authSource *auth.Source, request *http.Request, response http.ResponseWriter) (*user_model.User, goth.User, error) {
	gothUser, err := oAuth2FetchUser(ctx, authSource, request, response)
	if err != nil {
		return nil, goth.User{}, err
	}

	// >>> @@@ STACKIT CODE @@@
	// User Story 44186 and 51702
	bIsAdmin := false
	if gothUser.Provider == "STACKIT IDP" {
		orgID, _ := getOrganizationID(ctx, gothUser)
		bIsAdmin, err = oauth2CheckOrganization(ctx, authSource, gothUser, orgID)
		if err != nil {
			return nil, goth.User{}, err
			// It's not necessary promote anything the user has already been created as remote user in the previous step
		}
	} else {
		// Continue with the legacy behavior
		if _, _, err := remote_service.MaybePromoteRemoteUser(ctx, authSource, gothUser.UserID, gothUser.Email); err != nil {
			return nil, goth.User{}, err
		}
	}
	// <<< @@@ STACKIT CODE @@@
	// jira task STACKITGIT-867
	err = tryToCreateOrUpdateUser(ctx, gothUser, authSource)
	if err != nil {
		return nil, goth.User{}, err
	}
	// end of jira task STACKITGIT-867
	// <<< @@@ STACKIT CODE @@@

	u, err := oAuth2GothUserToUser(request.Context(), authSource, gothUser)
	// <<< @@@ STACKIT CODE @@@
	if err != nil {
		return nil, goth.User{}, err
	}
	// <<< @@@ STACKIT CODE @@@

	if u == nil {
		return nil, goth.User{}, user_model.ErrUserProhibitLogin{UID: 0, Name: gothUser.Email}
	}

	// >>> @@@ STACKIT CODE @@@
	// STACKITGIT-256 If the user is already admin then it will remain as admin
	u.IsAdmin = bIsAdmin || u.IsAdmin
	err = user_model.UpdateUserCols(ctx, u, "is_admin")
	if err != nil {
		return nil, goth.User{}, err
	}
	// <<< @@@ STACKIT CODE @@@

	return u, gothUser, err
}

func oAuth2FetchUser(ctx *context.Context, authSource *auth.Source, request *http.Request, response http.ResponseWriter) (goth.User, error) {
	oauth2Source := authSource.Cfg.(*oauth2.Source)

	// Make sure that the response is not an error response.
	errorName := request.FormValue("error")

	if len(errorName) > 0 {
		errorDescription := request.FormValue("error_description")

		// Delete the goth session
		err := gothic.Logout(response, request)
		if err != nil {
			return goth.User{}, err
		}

		return goth.User{}, errCallback{
			Code:        errorName,
			Description: errorDescription,
		}
	}

	// Proceed to authenticate through goth.
	codeVerifier, _ := ctx.Session.Get("CodeVerifier").(string)
	_ = ctx.Session.Delete("CodeVerifier")
	gothUser, err := oauth2Source.Callback(request, response, codeVerifier)
	if err != nil {
		if err.Error() == "securecookie: the value is too long" || strings.Contains(err.Error(), "Data too long") {
			log.Error("OAuth2 Provider %s returned too long a token. Current max: %d. Either increase the [OAuth2] MAX_TOKEN_LENGTH or reduce the information returned from the OAuth2 provider", authSource.Name, setting.OAuth2.MaxTokenLength)
			err = fmt.Errorf("OAuth2 Provider %s returned too long a token. Current max: %d. Either increase the [OAuth2] MAX_TOKEN_LENGTH or reduce the information returned from the OAuth2 provider", authSource.Name, setting.OAuth2.MaxTokenLength)
		}
		return goth.User{}, err
	}

	if oauth2Source.RequiredClaimName != "" {
		claimInterface, has := gothUser.RawData[oauth2Source.RequiredClaimName]
		if !has {
			return goth.User{}, user_model.ErrUserProhibitLogin{Name: gothUser.UserID}
		}

		if oauth2Source.RequiredClaimValue != "" {
			groups := claimValueToStringSet(claimInterface)

			if !groups.Contains(oauth2Source.RequiredClaimValue) {
				return goth.User{}, user_model.ErrUserProhibitLogin{Name: gothUser.UserID}
			}
		}
	}

	// >>> @@@ Stackit Workarround

	// Not all the IDPs provide the email in the raw data.
	// In this case we will check if the Name, LastName, FirstName and NickName fields are available and any of them can be used as email
	if gothUser.Email == "" {
		emailSources := []string{gothUser.Name, gothUser.LastName, gothUser.FirstName, gothUser.NickName}
		for _, source := range emailSources {
			if validation.ValidateEmail(source) == nil {
				gothUser.Email = source
				break // Found a valid email, no need to check further
			}
		}
	}

	// Impossible to retrieve the email from the raw data.
	if gothUser.Email == "" {
		log.Error("Failed to retrieve email from raw data for user %s", gothUser.UserID)
		return goth.User{}, user_model.ErrUserProhibitLogin{Name: gothUser.UserID}
	}

	gothUser.UserID = gothUser.Email
	// <<< @@@ Stackit Workarround

	return gothUser, nil
}

func oAuth2GothUserToUser(ctx go_context.Context, authSource *auth.Source, gothUser goth.User) (*user_model.User, error) {
	user := &user_model.User{
		LoginName:   gothUser.UserID,
		LoginType:   auth.OAuth2,
		LoginSource: authSource.ID,
	}

	hasUser, err := user_model.GetUser(ctx, user)
	if err != nil {
		return nil, err
	}

	if hasUser {
		return user, nil
	}
	log.Debug("no user found for LoginName %v, LoginSource %v, LoginType %v", user.LoginName, user.LoginSource, user.LoginType)

	// search in external linked users
	externalLoginUser := &user_model.ExternalLoginUser{
		ExternalID:    gothUser.UserID,
		LoginSourceID: authSource.ID,
	}
	hasUser, err = user_model.GetExternalLogin(ctx, externalLoginUser)
	if err != nil {
		return nil, err
	}
	if hasUser {
		user, err = user_model.GetUserByID(ctx, externalLoginUser.UserID)
		return user, err
	}

	// no user found to login
	return nil, nil
}

// <<< @@@ STACKIT CODE @@@
// TO BE DELETE IN 3 MONTHS after 24/05/2025
func manageDigitsMailExtension(ctx *context.Context, gothUser goth.User, username string) (bool, error) {
	newEmail := gothUser.Email
	log.Debug("manageDigitsMailExtension: gothUser=%+v, userName=%s", gothUser, newEmail)
	// Get the user by the old mail from the database. If that does not exist, then the process continues.
	// We don't know if the actual digits or external.digits comes from mail.schwarz or stackit.cloud
	// Lets try to load all the posibilities in one shot with remote_service.GetUsersInMails(ctx, username@mail.schwarz, username@external.mail.schwarz,username@stackit.cloud,username@external.stackit.cloud)

	users, err := remote_service.GetUsersByMail(ctx, username+"@"+OldMailExtensions[0], username+"@"+OldMailExtensions[1], username+"@"+OldMailExtensions[2], username+"@"+OldMailExtensions[3])
	if err != nil {
		return false, err
	}
	// iterate all users
	for _, user := range users {
		// If the actual user in DB has the digits.schwars suffix, we do nothing here
		if !strings.HasSuffix(user.LoginName, BaseNewExtension) {
			log.Warn("We are starting migration to digits.schwarz. User: %+v to gothUser  %+v", user.LoginName, gothUser.Email)
			// Check if the user mail extension has match with any of the OldMailExtensions
			for _, oldExtension := range OldMailExtensions {
				// if the user mail extension has match with any of the OldMailExtensions
				if strings.HasSuffix(user.LoginName, oldExtension) {
					// We update all the attributes of the user with this email

					log.Warn("Updating login_name in user.ID: %+v with login_name %+v", user.ID, newEmail)
					user.LoginName = newEmail
					log.Warn("Updating email in user.ID: %+v with email %+v", user.ID, newEmail)
					user.Email = newEmail
					log.Warn("Updating avatar_email in user.ID: %+v with avatar_email %+v", user.ID, newEmail)
					user.AvatarEmail = newEmail

					err = user_model.UpdateUserCols(ctx, user, "login_name", "email", "avatar_email")
					if err != nil {
						log.Error("manageDigitsMailExtension updating user in user.ID: %+v with email %+v", user.ID, newEmail)
						return false, err
					}

					log.Warn("Inserting new EmailAddress in user.ID: %+v with email %+v", user.ID, newEmail)
					// Insert new address
					email := &user_model.EmailAddress{
						UID:         user.ID,
						Email:       newEmail,
						IsActivated: true,
						IsPrimary:   true,
					}
					if _, err := user_model.InsertEmailAddress(ctx, email); err != nil {
						log.Error("manageDigitsMailExtension inserting email address in user.ID: %+v with email %+v", user.ID, newEmail)
						return false, err
					}
					emails, err := user_model.GetEmailAddresses(ctx, user.ID)
					if err != nil {
						log.Error("manageDigitsMailExtension GetEmailAddresses in user.ID: %+v with email %+v", user.ID, newEmail)
						return false, err
					}
					for _, email := range emails {
						isUserEmail := (email.Email == newEmail)
						// The email is primary and activated if the mail is the same as the goth user
						email.IsPrimary = isUserEmail
						email.IsActivated = isUserEmail
						log.Warn("Updating EmailAddresses in user.ID: %+v with email %+v", user.ID, email)
						err = user_model.UpdateEmailAddress(ctx, email)
						if err != nil {
							log.Error("manageDigitsMailExtension UpdateEmailAddress in user.ID: %+v with email %+v", user.ID, newEmail)
							return false, err
						}
					}
					// if the user is updated, then we return true and finish the method.
					return true, nil
				}
			}
		}
	}

	return false, nil
}

const BaseNewExtension = "digits.schwarz"

var OldMailExtensions = []string{"mail.schwarz", "external.mail.schwarz", "stackit.cloud", "external.stackit.cloud"}

// <<< @@@ STACKIT CODE @@@
// TO BE DELETE IN 3 MONTHS after 24/05/2025
// jira task STACKITGIT-867
// tryToCreateOrUpdateUser checks if a user from an OAuth provider exists locally.
// If the user has a new email domain (e.g., digits.schwarz), it attempts to migrate an existing user from an old domain.
// If no local user is found, it creates a new one.
//
// Parameters:
//   - ctx: The context for the request and database operations.
//   - gothUser: The user information retrieved from the OAuth provider.
//   - authSource: The authentication source used for login.
//
// Returns:
//   - error: An error if user creation or update fails, otherwise nil.
func tryToCreateOrUpdateUser(ctx *context.Context, gothUser goth.User, authSource *auth.Source) error {
	// Use the leftmost part of the email as user name

	emailspt := strings.SplitN(gothUser.Email, "@", 2)
	userName := emailspt[0]
	currentMailExtension := emailspt[1]

	// if the currentMailExtension split contains digits.schwarz
	if strings.HasSuffix(currentMailExtension, BaseNewExtension) {
		updated, err := manageDigitsMailExtension(ctx, gothUser, userName)
		if err != nil {
			return err
		}
		if updated {
			return nil
		}
	}
	// if not we do what we do here

	// The gothUser.UserID user id is taken from the IDP login (name@domain.ext) and tranformed as name.hash as login name
	calculatedLogin, err := user_model.GenerateCalculatedLogin(ctx, gothUser.UserID)
	if err != nil {
		// There is an error getting the users
		return err
	}

	users, err := remote_service.GetUsersByLoginName(ctx, userName, calculatedLogin, gothUser.UserID)
	if err != nil {
		// There is an error getting the users
		return err
	}

	if len(users) == 0 {
		// The user doesn't exist in the local users, so create a new user

		// When creating the local user name the IsUsableUserName tries to do a first check of the syntax.
		// In a short summary it checks the following patterns `^[\da-zA-Z][-.\w]* plus the list of reserved words
		err = createLocalUser(ctx, authSource, gothUser, calculatedLogin)
		if err != nil {
			return err // The correct error is already returned by createLocalUser
		}
	}

	return nil
}

// <<< @@@ STACKIT CODE @@@
// TO BE ROLLED BACK IN 3 MONTHS after 24/05/2025
// jira task STACKITGIT-867
// func createUser(ctx *context.Context, gothUser goth.User, authSource *auth.Source) error {
// 	// Use the leftmost part of the email as user name
// 	emailspt := strings.SplitN(gothUser.Email, "@", 2)
// 	userName := emailspt[0]

// 	// The gothUser.UserID user id is taken from the IDP login (name@domain.ext) and tranformed as name.hash as login name
// 	calculatedLogin, err := user_model.GenerateCalculatedLogin(ctx, gothUser.UserID)
// 	if err != nil {
// 		// There is an error getting the users
// 		return err
// 	}

// 	users, err := remote_service.GetUsersByLoginName(ctx, userName, calculatedLogin, gothUser.UserID)
// 	if err != nil {
// 		// There is an error getting the users
// 		return err
// 	}

// 	if len(users) == 0 {
// 		// The user doesn't exist in the local users, so create a new user

// 		// When creating the local user name the IsUsableUserName tries to do a first check of the syntax.
// 		// In a short summary it checks the following patterns `^[\da-zA-Z][-.\w]* plus the list of reserved words
// 		err = createLocalUser(ctx, authSource, gothUser, calculatedLogin)
// 		if err != nil {
// 			return err // The correct error is already returned by createLocalUser
// 		}

// 	}

// 	return nil
// }
