package admin

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAuthenticatorLoginSessionAndCSRF(t *testing.T) {
	t.Setenv("FRP_ADMIN_USERNAME", "admin")
	t.Setenv("FRP_ADMIN_PASSWORD", "test-password")
	auth, err := NewAuthenticator(AuthOptions{
		DataDir:    t.TempDir(),
		Role:       "server",
		SessionTTL: time.Hour,
	})
	require.NoError(t, err)

	login := httptest.NewRequest(http.MethodPost, "/api/v2/auth/login", strings.NewReader(`{"username":"admin","password":"test-password"}`))
	login.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	auth.Login(recorder, login)
	require.Equal(t, http.StatusOK, recorder.Code)
	cookie := recorder.Result().Cookies()[0]

	var delivered bool
	handler := auth.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		delivered = true
		w.WriteHeader(http.StatusNoContent)
	}))

	sessionRequest := httptest.NewRequest(http.MethodGet, "/api/v2/auth/session", nil)
	sessionRequest.AddCookie(cookie)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, sessionRequest)
	require.Equal(t, http.StatusNoContent, recorder.Code)
	require.True(t, delivered)

	delivered = false
	writeRequest := httptest.NewRequest(http.MethodPost, "/api/v2/config/file", strings.NewReader(`{}`))
	writeRequest.AddCookie(cookie)
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, writeRequest)
	require.Equal(t, http.StatusForbidden, recorder.Code)
	require.False(t, delivered)
}

func TestAuthenticatorRejectsInvalidPassword(t *testing.T) {
	t.Setenv("FRP_ADMIN_PASSWORD", "test-password")
	auth, err := NewAuthenticator(AuthOptions{DataDir: t.TempDir(), Role: "client"})
	require.NoError(t, err)

	login := httptest.NewRequest(http.MethodPost, "/api/v2/auth/login", strings.NewReader(`{"username":"admin","password":"wrong"}`))
	login.RemoteAddr = "127.0.0.1:12345"
	recorder := httptest.NewRecorder()
	auth.Login(recorder, login)
	require.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestAuthenticatorRequiresBootstrapPassword(t *testing.T) {
	t.Setenv("FRP_ADMIN_PASSWORD", "")
	_, err := NewAuthenticator(AuthOptions{DataDir: t.TempDir(), Role: "server"})
	require.ErrorContains(t, err, "FRP_ADMIN_PASSWORD")
}

func TestNewSessionCookieAttributes(t *testing.T) {
	tests := []struct {
		name           string
		target         string
		forwardedHTTPS bool
		secureOption   bool
		wantSecure     bool
	}{
		{
			name:       "http",
			target:     "http://example.com/api/v2/auth/login",
			wantSecure: false,
		},
		{
			name:       "https",
			target:     "https://example.com/api/v2/auth/login",
			wantSecure: true,
		},
		{
			name:           "forwarded https",
			target:         "http://example.com/api/v2/auth/login",
			forwardedHTTPS: true,
			wantSecure:     true,
		},
		{
			name:         "secure option",
			target:       "http://example.com/api/v2/auth/login",
			secureOption: true,
			wantSecure:   true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, test.target, nil)
			if test.forwardedHTTPS {
				request.Header.Set("X-Forwarded-Proto", "HTTPS")
			}
			auth := &Authenticator{secure: test.secureOption}
			recorder := httptest.NewRecorder()

			auth.setSessionCookie(recorder, request, "token", time.Time{}, 3600)

			cookies := recorder.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie := cookies[0]
			require.Equal(t, sessionCookieName, cookie.Name)
			require.Equal(t, "token", cookie.Value)
			require.Equal(t, "/", cookie.Path)
			require.True(t, cookie.HttpOnly)
			require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			require.Equal(t, test.wantSecure, cookie.Secure)
		})
	}
}

func TestLogoutAndChangePasswordClearSessionCookie(t *testing.T) {
	t.Setenv("FRP_ADMIN_PASSWORD", "test-password")
	auth, err := NewAuthenticator(AuthOptions{DataDir: t.TempDir(), Role: "server"})
	require.NoError(t, err)

	logout := httptest.NewRequest(http.MethodPost, "https://example.com/api/v2/auth/logout", nil)
	recorder := httptest.NewRecorder()
	auth.Logout(recorder, logout)
	require.Equal(t, http.StatusNoContent, recorder.Code)

	changePassword := httptest.NewRequest(
		http.MethodPost,
		"https://example.com/api/v2/auth/password",
		strings.NewReader(`{"currentPassword":"test-password","newPassword":"new-test-password"}`),
	)
	changeRecorder := httptest.NewRecorder()
	auth.ChangePassword(changeRecorder, changePassword)
	require.Equal(t, http.StatusOK, changeRecorder.Code)

	for _, recorder := range []*httptest.ResponseRecorder{recorder, changeRecorder} {
		cookies := recorder.Result().Cookies()
		require.Len(t, cookies, 1)
		cookie := cookies[0]
		require.Equal(t, sessionCookieName, cookie.Name)
		require.Empty(t, cookie.Value)
		require.Equal(t, "/", cookie.Path)
		require.True(t, cookie.HttpOnly)
		require.True(t, cookie.Secure)
		require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		require.Equal(t, -1, cookie.MaxAge)
	}
}
