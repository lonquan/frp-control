// Copyright 2026 The frp Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package admin

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/argon2"

	httppkg "github.com/fatedier/frp/pkg/util/http"
)

const (
	sessionCookieName = "frp_admin_session"
	defaultSessionTTL = 7 * 24 * time.Hour
)

type Session struct {
	Token     string    `json:"token"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type authState struct {
	Username     string              `json:"username"`
	PasswordHash string              `json:"passwordHash"`
	Sessions     map[string]*Session `json:"sessions"`
}

type loginAttempt struct {
	Failures     int
	BlockedUntil time.Time
	LastFailure  time.Time
}

type Authenticator struct {
	mu       sync.Mutex
	path     string
	role     string
	ttl      time.Duration
	secure   bool
	state    authState
	attempts map[string]*loginAttempt
}

type AuthOptions struct {
	DataDir      string
	Role         string
	SessionTTL   time.Duration
	SecureCookie bool
}

type SessionResponse struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CSRFToken string    `json:"csrfToken"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func NewAuthenticator(opts AuthOptions) (*Authenticator, error) {
	if opts.DataDir == "" {
		return nil, errors.New("admin data directory is required")
	}
	if opts.Role == "" {
		opts.Role = "instance"
	}
	if opts.SessionTTL <= 0 {
		opts.SessionTTL = defaultSessionTTL
	}

	authDir := filepath.Join(opts.DataDir, "admin")
	if err := os.MkdirAll(authDir, 0o700); err != nil {
		return nil, fmt.Errorf("create admin data directory: %w", err)
	}
	a := &Authenticator{
		path:     filepath.Join(authDir, "auth.json"),
		role:     opts.Role,
		ttl:      opts.SessionTTL,
		secure:   opts.SecureCookie,
		attempts: make(map[string]*loginAttempt),
	}
	if err := a.load(); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Authenticator) Role() string {
	return a.role
}

func (a *Authenticator) Enabled() bool {
	return a != nil
}

func (a *Authenticator) load() error {
	b, err := os.ReadFile(a.path)
	if err == nil {
		if err := json.Unmarshal(b, &a.state); err != nil {
			return fmt.Errorf("parse admin auth state: %w", err)
		}
		if a.state.Sessions == nil {
			a.state.Sessions = make(map[string]*Session)
		}
		a.pruneSessionsLocked(time.Now())
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read admin auth state: %w", err)
	}

	username := strings.TrimSpace(os.Getenv("FRP_ADMIN_USERNAME"))
	if username == "" {
		username = "admin"
	}
	password := os.Getenv("FRP_ADMIN_PASSWORD")
	if password == "" {
		return errors.New("FRP_ADMIN_PASSWORD is required before the admin panel can start")
	}
	hash, err := hashPassword(password)
	if err != nil {
		return err
	}
	a.state = authState{
		Username:     username,
		PasswordHash: hash,
		Sessions:     make(map[string]*Session),
	}
	return a.saveLocked()
}

func (a *Authenticator) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "登录请求格式错误")
		return
	}
	key := clientIP(r) + "|" + strings.TrimSpace(body.Username)
	now := time.Now()

	a.mu.Lock()
	attempt := a.attempts[key]
	if attempt != nil && attempt.BlockedUntil.After(now) {
		until := attempt.BlockedUntil
		a.mu.Unlock()
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(time.Until(until).Seconds()))))
		writeError(w, http.StatusTooManyRequests, fmt.Sprintf("登录尝试过多，请于 %s 后重试", until.Format("15:04:05")))
		return
	}
	username := a.state.Username
	hash := a.state.PasswordHash
	a.mu.Unlock()

	if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(body.Username)), []byte(username)) != 1 ||
		!verifyPassword(hash, body.Password) {
		a.recordFailure(key)
		time.Sleep(200 * time.Millisecond)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}

	token, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建会话失败")
		return
	}
	csrf, err := randomToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "创建会话失败")
		return
	}
	session := &Session{
		Token:     token,
		CSRFToken: csrf,
		ExpiresAt: now.Add(a.ttl),
	}

	a.mu.Lock()
	delete(a.attempts, key)
	a.pruneSessionsLocked(now)
	a.state.Sessions[token] = session
	err = a.saveLocked()
	a.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存会话失败")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   a.secure || r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https"),
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
		MaxAge:   int(a.ttl.Seconds()),
	})
	writeJSON(w, http.StatusOK, SessionResponse{
		Username:  username,
		Role:      a.role,
		CSRFToken: csrf,
		ExpiresAt: session.ExpiresAt,
	})
}

func (a *Authenticator) Session(w http.ResponseWriter, r *http.Request) {
	session, ok := a.sessionFromRequest(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "登录状态已失效")
		return
	}
	a.mu.Lock()
	username := a.state.Username
	a.mu.Unlock()
	writeJSON(w, http.StatusOK, SessionResponse{
		Username:  username,
		Role:      a.role,
		CSRFToken: session.CSRFToken,
		ExpiresAt: session.ExpiresAt,
	})
}

func (a *Authenticator) Logout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	cookie, err := r.Cookie(sessionCookieName)
	if err == nil {
		a.mu.Lock()
		delete(a.state.Sessions, cookie.Value)
		_ = a.saveLocked()
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *Authenticator) ChangePassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "请求方法不支持")
		return
	}
	var body struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "密码请求格式错误")
		return
	}
	if len(body.NewPassword) < 8 {
		writeError(w, http.StatusUnprocessableEntity, "新密码至少需要 8 个字符")
		return
	}
	a.mu.Lock()
	if !verifyPassword(a.state.PasswordHash, body.CurrentPassword) {
		a.mu.Unlock()
		writeError(w, http.StatusUnauthorized, "当前密码错误")
		return
	}
	hash, err := hashPassword(body.NewPassword)
	if err != nil {
		a.mu.Unlock()
		writeError(w, http.StatusInternalServerError, "更新密码失败")
		return
	}
	a.state.PasswordHash = hash
	a.state.Sessions = make(map[string]*Session)
	err = a.saveLocked()
	a.mu.Unlock()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "保存密码失败")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]string{"message": "密码已更新，请重新登录"})
}

func (a *Authenticator) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, ok := a.sessionFromRequest(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "请先登录管理面板")
			return
		}
		if isMutating(r.Method) && !isSafeRead(r) {
			csrf := r.Header.Get("X-CSRF-Token")
			if subtle.ConstantTimeCompare([]byte(csrf), []byte(session.CSRFToken)) != 1 {
				writeError(w, http.StatusForbidden, "CSRF 校验失败，请刷新页面后重试")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Authenticator) sessionFromRequest(r *http.Request) (*Session, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return nil, false
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	session := a.state.Sessions[cookie.Value]
	if session == nil || !session.ExpiresAt.After(now) {
		if session != nil {
			delete(a.state.Sessions, cookie.Value)
			_ = a.saveLocked()
		}
		return nil, false
	}
	if time.Until(session.ExpiresAt) < a.ttl/2 {
		session.ExpiresAt = now.Add(a.ttl)
		_ = a.saveLocked()
	}
	copy := *session
	return &copy, true
}

func (a *Authenticator) UsernameFromRequest(r *http.Request) string {
	if _, ok := a.sessionFromRequest(r); !ok {
		return ""
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.state.Username
}

func (a *Authenticator) recordFailure(key string) {
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt := a.attempts[key]
	if attempt == nil || now.Sub(attempt.LastFailure) > 15*time.Minute {
		attempt = &loginAttempt{}
		a.attempts[key] = attempt
	}
	attempt.Failures++
	attempt.LastFailure = now
	if attempt.Failures >= 5 {
		attempt.BlockedUntil = now.Add(15 * time.Minute)
		attempt.Failures = 0
	}
}

func (a *Authenticator) pruneSessionsLocked(now time.Time) {
	if a.state.Sessions == nil {
		a.state.Sessions = make(map[string]*Session)
		return
	}
	for token, session := range a.state.Sessions {
		if session == nil || !session.ExpiresAt.After(now) {
			delete(a.state.Sessions, token)
		}
	}
}

func (a *Authenticator) saveLocked() error {
	b, err := json.MarshalIndent(a.state, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(a.path)
	tmp, err := os.CreateTemp(dir, ".auth-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, a.path)
}

func hashPassword(password string) (string, error) {
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	const (
		memory      = 64 * 1024
		iterations  = 3
		parallelism = 2
		keyLength   = 32
	)
	hash := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, keyLength)
	return fmt.Sprintf("argon2id$v=19$m=%d,t=%d,p=%d$%s$%s",
		memory,
		iterations,
		parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

func verifyPassword(encoded, password string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 5 || parts[0] != "argon2id" || parts[1] != "v=19" {
		return false
	}
	var memory uint32
	var iterations uint32
	var parallelism uint8
	if _, err := fmt.Sscanf(parts[2], "m=%d,t=%d,p=%d", &memory, &iterations, &parallelism); err != nil {
		return false
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[3])
	if err != nil {
		return false
	}
	expected, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	actual := argon2.IDKey([]byte(password), salt, iterations, memory, parallelism, uint32(len(expected)))
	return subtle.ConstantTimeCompare(actual, expected) == 1
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}
	return r.RemoteAddr
}

func isMutating(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	default:
		return true
	}
}

func isSafeRead(r *http.Request) bool {
	return r.Method == http.MethodGet && r.URL.Path == "/api/v2/auth/session"
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": status,
		"msg":  http.StatusText(status),
		"data": data,
	})
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code": status,
		"msg":  message,
		"data": nil,
	})
}

func RequireSession(auth *Authenticator, next http.Handler) http.Handler {
	if auth == nil {
		return next
	}
	return auth.Middleware(next)
}

func LegacyBasicMiddleware(user, password string) func(http.Handler) http.Handler {
	if user == "" && password == "" {
		return func(next http.Handler) http.Handler { return next }
	}
	expected := httppkg.BasicAuth(user, password)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte(expected)) != 1 {
				w.Header().Set("WWW-Authenticate", `Basic realm="frp"`)
				writeError(w, http.StatusUnauthorized, "需要 Basic Auth")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
