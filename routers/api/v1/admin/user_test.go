package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	user_model "forgejo.org/models/user"
	"forgejo.org/modules/structs"
	"forgejo.org/modules/timeutil"
	app_context "forgejo.org/services/context"

	"github.com/stretchr/testify/assert"
)

// mockResponseWriter wraps httptest.ResponseRecorder to satisfy Gitea's ResponseWriter interface
type mockResponseWriter struct {
	*httptest.ResponseRecorder
}

// WrittenStatus implements web_types.ResponseStatusProvider
func (m *mockResponseWriter) WrittenStatus() int {
	// Simulate not written yet
	return 0
}

// Status implements context.ResponseWriter
func (m *mockResponseWriter) Status() int {
	return m.Code
}

// Size implements context.ResponseWriter
func (m *mockResponseWriter) Size() int {
	return m.Body.Len()
}

// Before implements context.ResponseWriter
// We can leave this empty for unit tests as we aren't testing middleware chains
func (m *mockResponseWriter) Before(f func(app_context.ResponseWriter)) {
	// no-op
}

func TestCreateUser_Complete(t *testing.T) {
	// 1. Define Test Table
	type testCase struct {
		name           string
		forceResetPass *bool
		sendNotify     bool
		wantResetMail  bool
		wantNotifyMail bool
		wantHTTPStatus int
	}

	truePtr := true
	falsePtr := false

	tests := []testCase{
		{
			name:           "Force Reset Password",
			forceResetPass: &truePtr,
			sendNotify:     false,
			wantResetMail:  true,
			wantNotifyMail: false,
			wantHTTPStatus: http.StatusCreated,
		},
		{
			name:           "Send Notify Only",
			forceResetPass: &falsePtr,
			sendNotify:     true,
			wantResetMail:  false,
			wantNotifyMail: true,
			wantHTTPStatus: http.StatusCreated,
		},
		{
			name:           "Both True (Reset takes priority)",
			forceResetPass: &truePtr,
			sendNotify:     true,
			wantResetMail:  true,
			wantNotifyMail: false,
			wantHTTPStatus: http.StatusCreated,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// --- SETUP: Restore original functions after test ---
			origReset := sendResetPasswordMailFunc
			origNotify := sendRegisterNotifyMailFunc
			origCreate := adminCreateUserFunc
			origPwned := passwordIsPwnedFunc

			defer func() {
				sendResetPasswordMailFunc = origReset
				sendRegisterNotifyMailFunc = origNotify
				adminCreateUserFunc = origCreate
				passwordIsPwnedFunc = origPwned
			}()

			// --- MOCKING ---
			var resetCalled, notifyCalled bool

			// 1. Mock Mailers
			sendResetPasswordMailFunc = func(ctx context.Context, u *user_model.User) error {
				resetCalled = true
				return nil
			}
			sendRegisterNotifyMailFunc = func(u *user_model.User) { notifyCalled = true }

			// 2. Mock AdminCreateUser
			adminCreateUserFunc = func(ctx context.Context, u *user_model.User, overwriteDefault ...*user_model.CreateUserOverwriteOptions) error {
				// Simulate Database behavior:
				u.ID = -1

				// 2. Set default fields usually handled by DB/Logic if they are empty
				if u.CreatedUnix == 0 {
					u.CreatedUnix = timeutil.TimeStamp(time.Now().Unix())
				}
				u.UpdatedUnix = u.CreatedUnix

				// 3. Handle 'overwriteDefault' if you need to test that logic
				// (The actual function loops over this slice, for this test we can ignore it
				// or assert that it was passed correctly if strictly needed).
				return nil
			}

			// 3. Mock Password Check
			passwordIsPwnedFunc = func(ctx context.Context, p string) error { return nil }

			// --- DATA PREPARATION ---
			formOption := &structs.CreateUserOption{
				Username:               "Ghost",
				LoginName:              "user_" + tc.name,
				Email:                  "user@example.com",
				Password:               "ComplexPass123!",
				ForceSendResetPassword: tc.forceResetPass,
				SendNotify:             tc.sendNotify,
			}

			// --- CONTEXT CONSTRUCTION ---
			resp := httptest.NewRecorder()
			mockResp := &mockResponseWriter{
				ResponseRecorder: resp,
			}

			// Populate Context Data for web.GetForm
			ctxData := make(map[string]any)
			ctxData["__form"] = formOption

			innerCtx := &app_context.Base{
				Data: ctxData,
				// Dummy repo
				Resp: mockResp,
			}

			// Admin User performing the action
			doer := &user_model.User{
				ID:      1,
				Name:    "Admin",
				IsAdmin: true,
			}

			apiCtx := &app_context.APIContext{
				Base: innerCtx,
			}
			apiCtx.SetDoer(doer)

			// --- EXECUTE ---
			CreateUser(apiCtx)

			// --- ASSERTIONS ---
			assert.Equal(t, tc.wantHTTPStatus, resp.Code)

			// Verify Mail Logic
			assert.Equal(t, tc.wantResetMail, resetCalled, "Mismatch in Reset Mail expectation")
			assert.Equal(t, tc.wantNotifyMail, notifyCalled, "Mismatch in Notify Mail expectation")
		})
	}
}
