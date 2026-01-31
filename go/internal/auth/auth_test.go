package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewTokenManager(t *testing.T) {
	tm := NewTokenManager("test-secret")
	if tm == nil {
		t.Fatal("Expected TokenManager, got nil")
	}
	if string(tm.secretKey) != "test-secret" {
		t.Errorf("Expected secretKey 'test-secret', got '%s'", string(tm.secretKey))
	}
	if tm.tokenDuration != 1*time.Hour {
		t.Errorf("Expected tokenDuration 1h, got %v", tm.tokenDuration)
	}
}

func TestGenerateToken(t *testing.T) {
	tm := NewTokenManager("test-secret")
	token, err := tm.GenerateToken("user123", "wallet456", []Permission{PermissionReadOnly, PermissionReadWrite})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}
	if token == "" {
		t.Error("Expected non-empty token")
	}
}

func TestValidateToken(t *testing.T) {
	tm := NewTokenManager("test-secret")
	permissions := []Permission{PermissionReadOnly, PermissionReadWrite}

	token, err := tm.GenerateToken("user123", "wallet456", permissions)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	claims, err := tm.ValidateToken(token)
	if err != nil {
		t.Fatalf("Failed to validate token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}
	if claims.WalletAddr != "wallet456" {
		t.Errorf("Expected WalletAddr 'wallet456', got '%s'", claims.WalletAddr)
	}
	if len(claims.Permissions) != 2 {
		t.Errorf("Expected 2 permissions, got %d", len(claims.Permissions))
	}
}

func TestValidateTokenInvalid(t *testing.T) {
	tm := NewTokenManager("test-secret")

	// Test invalid token
	_, err := tm.ValidateToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}

	// Test token with wrong secret
	tm2 := NewTokenManager("wrong-secret")
	_, err = tm2.ValidateToken("eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoidXNlcjEyMyIsIndhbGxldF9hZGRyIjoid2FsbGV0NDU2IiwicGVybWlzc2lvbnMiOlsicmVhZCJdfQ.invalid")
	if err == nil {
		t.Error("Expected error for token with wrong secret")
	}
}

func TestRefreshToken(t *testing.T) {
	tm := NewTokenManager("test-secret")
	permissions := []Permission{PermissionReadOnly}

	oldToken, err := tm.GenerateToken("user123", "wallet456", permissions)
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	newToken, err := tm.RefreshToken(oldToken)
	if err != nil {
		t.Fatalf("Failed to refresh token: %v", err)
	}

	// Note: Tokens might be the same if generated at the exact same time
	// Just validate that the new token works
	claims, err := tm.ValidateToken(newToken)
	if err != nil {
		t.Fatalf("Failed to validate refreshed token: %v", err)
	}

	if claims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
	}
	if claims.WalletAddr != "wallet456" {
		t.Errorf("Expected WalletAddr 'wallet456', got '%s'", claims.WalletAddr)
	}
}

func TestRefreshTokenInvalid(t *testing.T) {
	tm := NewTokenManager("test-secret")

	_, err := tm.RefreshToken("invalid-token")
	if err == nil {
		t.Error("Expected error for invalid token refresh")
	}
}

func TestClaimsHasPermission(t *testing.T) {
	claims := &Claims{
		Permissions: []Permission{PermissionReadOnly, PermissionReadWrite},
	}

	// Test existing permission
	if !claims.HasPermission(PermissionReadOnly) {
		t.Error("Expected to have read permission")
	}

	// Test non-existing permission
	if claims.HasPermission(PermissionAdmin) {
		t.Error("Expected not to have admin permission")
	}

	// Test admin permission grants all
	adminClaims := &Claims{
		Permissions: []Permission{PermissionAdmin},
	}
	if !adminClaims.HasPermission(PermissionReadOnly) {
		t.Error("Expected admin to have read permission")
	}
	if !adminClaims.HasPermission(PermissionReadWrite) {
		t.Error("Expected admin to have write permission")
	}
	if !adminClaims.HasPermission(PermissionAdmin) {
		t.Error("Expected admin to have admin permission")
	}
}

func TestNewAuthMiddleware(t *testing.T) {
	tm := NewTokenManager("test-secret")
	middleware := NewAuthMiddleware(tm)
	if middleware == nil {
		t.Fatal("Expected AuthMiddleware, got nil")
	}
	if middleware.tokenManager != tm {
		t.Error("Expected tokenManager to be set")
	}
}

func TestAuthMiddlewareAuthenticate(t *testing.T) {
	tm := NewTokenManager("test-secret")
	middleware := NewAuthMiddleware(tm)

	// Create a valid token
	token, err := tm.GenerateToken("user123", "wallet456", []Permission{PermissionReadOnly})
	if err != nil {
		t.Fatalf("Failed to generate token: %v", err)
	}

	// Test with valid token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	called := false
	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		claims, ok := GetClaims(r.Context())
		if !ok {
			t.Error("Expected claims in context")
		}
		if claims.UserID != "user123" {
			t.Errorf("Expected UserID 'user123', got '%s'", claims.UserID)
		}
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if !called {
		t.Error("Expected handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestAuthMiddlewareAuthenticateMissingHeader(t *testing.T) {
	tm := NewTokenManager("test-secret")
	middleware := NewAuthMiddleware(tm)

	req := httptest.NewRequest("GET", "/test", nil)

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAuthenticateInvalidFormat(t *testing.T) {
	tm := NewTokenManager("test-secret")
	middleware := NewAuthMiddleware(tm)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "InvalidFormat token")

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestAuthMiddlewareAuthenticateInvalidToken(t *testing.T) {
	tm := NewTokenManager("test-secret")
	middleware := NewAuthMiddleware(tm)

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")

	handler := middleware.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Handler should not be called")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestGetClaims(t *testing.T) {
	claims := &Claims{UserID: "user123"}
	ctx := context.WithValue(context.Background(), claimsKey, claims)

	retrievedClaims, ok := GetClaims(ctx)
	if !ok {
		t.Error("Expected to retrieve claims")
	}
	if retrievedClaims.UserID != "user123" {
		t.Errorf("Expected UserID 'user123', got '%s'", retrievedClaims.UserID)
	}

	// Test with no claims
	_, ok = GetClaims(context.Background())
	if ok {
		t.Error("Expected not to retrieve claims from empty context")
	}
}
