package auth

import (
	"context"
	"net/http"
	"testing"
)

// newRequest is a test helper that creates an http.Request with context.
func newRequest(method, url string) *http.Request {
	req, _ := http.NewRequestWithContext(context.Background(), method, url, nil)
	return req
}

func TestNoneAuth(t *testing.T) {
	auth := NewNoneAuth()

	req := newRequest("POST", "/test")
	if !auth.Authenticate(req) {
		t.Error("NoneAuth should always authenticate")
	}
}

func TestBasicAuth(t *testing.T) {
	users := map[string]string{
		"admin": "password123",
		"user":  "secret",
	}
	auth := NewBasicAuth(users)

	tests := []struct {
		name     string
		username string
		password string
		want     bool
	}{
		{"valid credentials", "admin", "password123", true},
		{"valid user different password", "admin", "wrong", false},
		{"invalid user", "unknown", "password123", false},
		{"empty credentials", "", "", false},
		{"valid second user", "user", "secret", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("POST", "/test")
			req.SetBasicAuth(tt.username, tt.password)

			if got := auth.Authenticate(req); got != tt.want {
				t.Errorf("BasicAuth.Authenticate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBasicAuth_MissingHeader(t *testing.T) {
	auth := NewBasicAuth(map[string]string{"admin": "password"})
	req := newRequest("POST", "/test")

	if auth.Authenticate(req) {
		t.Error("Should fail with missing Authorization header")
	}
}

func TestBasicAuth_WrongScheme(t *testing.T) {
	auth := NewBasicAuth(map[string]string{"admin": "password"})
	req := newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer token123")

	if auth.Authenticate(req) {
		t.Error("Should fail with Bearer token when Basic expected")
	}
}

func TestBearerAuth(t *testing.T) {
	tokens := []string{"token1", "token2", "secret-key"}
	auth := NewBearerAuth(tokens)

	tests := []struct {
		name  string
		token string
		want  bool
	}{
		{"valid token 1", "token1", true},
		{"valid token 2", "token2", true},
		{"valid token 3", "secret-key", true},
		{"invalid token", "invalid", false},
		{"empty token", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := newRequest("POST", "/test")
			req.Header.Set("Authorization", "Bearer "+tt.token)

			if got := auth.Authenticate(req); got != tt.want {
				t.Errorf("BearerAuth.Authenticate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBearerAuth_CaseInsensitive(t *testing.T) {
	auth := NewBearerAuth([]string{"token123"})

	tests := []string{"Bearer token123", "bearer token123", "BEARER token123"}
	for _, authHeader := range tests {
		req := newRequest("POST", "/test")
		req.Header.Set("Authorization", authHeader)

		if !auth.Authenticate(req) {
			t.Errorf("Should accept token with case-insensitive Bearer: %s", authHeader)
		}
	}
}

func TestBearerAuth_MissingHeader(t *testing.T) {
	auth := NewBearerAuth([]string{"token"})
	req := newRequest("POST", "/test")

	if auth.Authenticate(req) {
		t.Error("Should fail with missing Authorization header")
	}
}

func TestBearerAuth_WrongFormat(t *testing.T) {
	auth := NewBearerAuth([]string{"token"})
	req := newRequest("POST", "/test")
	req.Header.Set("Authorization", "Basic credentials")

	if auth.Authenticate(req) {
		t.Error("Should fail with Basic auth when Bearer expected")
	}
}

func TestBearerAuth_NoSpaceAfterBearer(t *testing.T) {
	auth := NewBearerAuth([]string{"token"})
	req := newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearertoken")

	if auth.Authenticate(req) {
		t.Error("Should fail with malformed Bearer header")
	}
}

// -------------------------------------------------------------------
// MultiAuth
// -------------------------------------------------------------------

func TestMultiAuth_NoAuth(t *testing.T) {
	m := NewMultiAuth(nil, nil)
	if m.HasAuth() {
		t.Error("MultiAuth with no users/tokens should report HasAuth()=false")
	}

	req := newRequest("POST", "/test")
	if !m.Authenticate(req) {
		t.Error("MultiAuth with no auth should allow all requests")
	}
}

func TestMultiAuth_BasicOnly(t *testing.T) {
	m := NewMultiAuth(map[string]string{"admin": "password"}, nil)
	if !m.HasAuth() {
		t.Error("MultiAuth with users should report HasAuth()=true")
	}

	// Valid basic creds.
	req := newRequest("POST", "/test")
	req.SetBasicAuth("admin", "password")
	if !m.Authenticate(req) {
		t.Error("Should accept valid basic credentials")
	}

	// Invalid basic creds.
	req = newRequest("POST", "/test")
	req.SetBasicAuth("admin", "wrong")
	if m.Authenticate(req) {
		t.Error("Should reject invalid basic credentials")
	}

	// Bearer token when only basic configured → reject.
	req = newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer sometoken")
	if m.Authenticate(req) {
		t.Error("Should reject bearer token when only basic is configured")
	}

	// No header → reject.
	req = newRequest("POST", "/test")
	if m.Authenticate(req) {
		t.Error("Should reject request with no Authorization header when auth is configured")
	}
}

func TestMultiAuth_BearerOnly(t *testing.T) {
	m := NewMultiAuth(nil, []string{"token123"})
	if !m.HasAuth() {
		t.Error("MultiAuth with tokens should report HasAuth()=true")
	}

	// Valid bearer token.
	req := newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer token123")
	if !m.Authenticate(req) {
		t.Error("Should accept valid bearer token")
	}

	// Invalid bearer token.
	req = newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer wrong")
	if m.Authenticate(req) {
		t.Error("Should reject invalid bearer token")
	}

	// Basic creds when only bearer configured → reject.
	req = newRequest("POST", "/test")
	req.SetBasicAuth("admin", "password")
	if m.Authenticate(req) {
		t.Error("Should reject basic credentials when only bearer is configured")
	}
}

func TestMultiAuth_Both(t *testing.T) {
	m := NewMultiAuth(map[string]string{"admin": "password"}, []string{"token123"})
	if !m.HasAuth() {
		t.Error("MultiAuth with both should report HasAuth()=true")
	}

	// Valid basic.
	req := newRequest("POST", "/test")
	req.SetBasicAuth("admin", "password")
	if !m.Authenticate(req) {
		t.Error("Should accept valid basic credentials when both configured")
	}

	// Valid bearer.
	req = newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer token123")
	if !m.Authenticate(req) {
		t.Error("Should accept valid bearer token when both configured")
	}

	// Invalid basic.
	req = newRequest("POST", "/test")
	req.SetBasicAuth("admin", "wrong")
	if m.Authenticate(req) {
		t.Error("Should reject invalid basic credentials when both configured")
	}

	// Invalid bearer.
	req = newRequest("POST", "/test")
	req.Header.Set("Authorization", "Bearer wrong")
	if m.Authenticate(req) {
		t.Error("Should reject invalid bearer token when both configured")
	}

	// Unknown scheme.
	req = newRequest("POST", "/test")
	req.Header.Set("Authorization", "Digest abc123")
	if m.Authenticate(req) {
		t.Error("Should reject unknown auth scheme")
	}

	// No header.
	req = newRequest("POST", "/test")
	if m.Authenticate(req) {
		t.Error("Should reject request with no Authorization header")
	}
}
