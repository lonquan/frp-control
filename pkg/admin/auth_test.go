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
