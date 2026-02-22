package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Authenticator is the interface all auth strategies must implement.
type Authenticator interface {
	Authenticate(r *http.Request) bool
}

// NoneAuth allows all requests.
type NoneAuth struct{}

func NewNoneAuth() *NoneAuth {
	return &NoneAuth{}
}

func (a *NoneAuth) Authenticate(r *http.Request) bool {
	return true
}

// BasicAuth validates HTTP Basic credentials against a username→password map.
type BasicAuth struct {
	users map[string]string
}

func NewBasicAuth(users map[string]string) *BasicAuth {
	return &BasicAuth{users: users}
}

func (a *BasicAuth) Authenticate(r *http.Request) bool {
	username, password, ok := r.BasicAuth()
	if !ok {
		return false
	}

	expectedPassword, exists := a.users[username]
	if !exists {
		return false
	}

	return subtle.ConstantTimeCompare([]byte(password), []byte(expectedPassword)) == 1
}

// BearerAuth validates Bearer tokens against a configured list.
type BearerAuth struct {
	tokens []string
}

func NewBearerAuth(tokens []string) *BearerAuth {
	// Store a copy so the caller can't mutate our slice.
	t := make([]string, len(tokens))
	copy(t, tokens)
	return &BearerAuth{tokens: t}
}

func (a *BearerAuth) Authenticate(r *http.Request) bool {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
		return false
	}

	incoming := []byte(parts[1])

	// Iterate over all stored tokens using constant-time comparison to prevent
	// timing side-channel attacks. We must not short-circuit on the first match
	// in a way that leaks information about which token matched.
	var matched int
	for _, t := range a.tokens {
		matched |= subtle.ConstantTimeCompare(incoming, []byte(t))
	}
	return matched == 1
}

// MultiAuth auto-detects the authentication scheme from the incoming
// Authorization header and delegates to BasicAuth or BearerAuth accordingly.
// If no users and no tokens are configured it allows all requests (like NoneAuth).
type MultiAuth struct {
	basic  *BasicAuth  // nil when no users configured
	bearer *BearerAuth // nil when no tokens configured
}

// NewMultiAuth creates an auto-detecting authenticator.
// Pass nil/empty maps or slices for schemes you don't need.
func NewMultiAuth(users map[string]string, tokens []string) *MultiAuth {
	m := &MultiAuth{}
	if len(users) > 0 {
		m.basic = NewBasicAuth(users)
	}
	if len(tokens) > 0 {
		m.bearer = NewBearerAuth(tokens)
	}
	return m
}

// HasAuth returns true if at least one auth scheme is configured.
func (m *MultiAuth) HasAuth() bool {
	return m.basic != nil || m.bearer != nil
}

// Authenticate inspects the Authorization header scheme and delegates.
// - No auth configured → allow everything.
// - "Basic ..." → delegate to BasicAuth (if configured).
// - "Bearer ..." → delegate to BearerAuth (if configured).
// - Missing/unrecognised header → reject when any auth is configured.
func (m *MultiAuth) Authenticate(r *http.Request) bool {
	if !m.HasAuth() {
		return true
	}

	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return false
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 {
		return false
	}

	switch strings.ToLower(parts[0]) {
	case "basic":
		if m.basic != nil {
			return m.basic.Authenticate(r)
		}
		return false
	case "bearer":
		if m.bearer != nil {
			return m.bearer.Authenticate(r)
		}
		return false
	default:
		return false
	}
}


