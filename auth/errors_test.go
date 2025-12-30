package auth

import (
	"testing"
)

func TestNewAPIError(t *testing.T) {
	err := NewAPIError(ErrInvalidRequest, "Test message", 400)

	if err.Code != ErrInvalidRequest {
		t.Errorf("Expected code ErrInvalidRequest, got %s", err.Code)
	}

	if err.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", err.Message)
	}

	if err.StatusCode != 400 {
		t.Errorf("Expected status code 400, got %d", err.StatusCode)
	}
}

func TestAPIError_Error(t *testing.T) {
	err := NewAPIError(ErrInvalidClient, "Invalid credentials", 401)
	errStr := err.Error()

	if errStr == "" {
		t.Errorf("Error() returned empty string")
	}

	if !contains(errStr, "invalid_client") {
		t.Errorf("Error() should contain error code, got %s", errStr)
	}
}

func TestAPIError_WithDetails(t *testing.T) {
	details := "Field 'email' is required"
	err := NewAPIError(ErrValidationFailed, "Validation failed", 400).WithDetails(details)

	if err.Details != details {
		t.Errorf("Expected details '%s', got '%s'", details, err.Details)
	}
}

func TestAPIError_WithOriginalError(t *testing.T) {
	originalErr := newTestError("original error")
	err := NewAPIError(ErrInternalServer, "Internal error", 500).WithOriginalError(originalErr)

	if err.originalErr == nil {
		t.Errorf("Expected originalErr to be set")
	}
}

func TestErrBadRequest(t *testing.T) {
	err := ErrBadRequest("Bad request message")

	if err.Code != ErrInvalidRequest {
		t.Errorf("Expected code ErrInvalidRequest, got %s", err.Code)
	}

	if err.StatusCode != 400 {
		t.Errorf("Expected status code 400, got %d", err.StatusCode)
	}
}

func TestErrUnauthorizedError(t *testing.T) {
	err := ErrUnauthorizedError("Unauthorized message")

	if err.Code != ErrUnauthorized {
		t.Errorf("Expected code ErrUnauthorized, got %s", err.Code)
	}

	if err.StatusCode != 401 {
		t.Errorf("Expected status code 401, got %d", err.StatusCode)
	}
}

func TestErrForbiddenError(t *testing.T) {
	err := ErrForbiddenError("Forbidden message")

	if err.Code != ErrForbidden {
		t.Errorf("Expected code ErrForbidden, got %s", err.Code)
	}

	if err.StatusCode != 403 {
		t.Errorf("Expected status code 403, got %d", err.StatusCode)
	}
}

func TestErrNotFoundError(t *testing.T) {
	err := ErrNotFoundError("Not found message")

	if err.Code != ErrNotFound {
		t.Errorf("Expected code ErrNotFound, got %s", err.Code)
	}

	if err.StatusCode != 404 {
		t.Errorf("Expected status code 404, got %d", err.StatusCode)
	}
}

func TestErrConflictError(t *testing.T) {
	err := ErrConflictError("Conflict message")

	if err.Code != ErrConflict {
		t.Errorf("Expected code ErrConflict, got %s", err.Code)
	}

	if err.StatusCode != 409 {
		t.Errorf("Expected status code 409, got %d", err.StatusCode)
	}
}

func TestErrInternalServerError(t *testing.T) {
	err := ErrInternalServerError("Internal error message")

	if err.Code != ErrInternalServer {
		t.Errorf("Expected code ErrInternalServer, got %s", err.Code)
	}

	if err.StatusCode != 500 {
		t.Errorf("Expected status code 500, got %d", err.StatusCode)
	}
}

func TestErrServiceUnavailableError(t *testing.T) {
	err := ErrServiceUnavailableError("Service unavailable")

	if err.Code != ErrServiceUnavailable {
		t.Errorf("Expected code ErrServiceUnavailable, got %s", err.Code)
	}

	if err.StatusCode != 503 {
		t.Errorf("Expected status code 503, got %d", err.StatusCode)
	}
}

func TestErrorCodes(t *testing.T) {
	tests := []struct {
		code       ErrorCode
		expectCode ErrorCode
	}{
		{ErrInvalidRequest, "invalid_request"},
		{ErrInvalidClient, "invalid_client"},
		{ErrInvalidGrant, "invalid_grant"},
		{ErrInvalidScope, "invalid_scope"},
		{ErrUnauthorized, "unauthorized"},
		{ErrForbidden, "forbidden"},
		{ErrNotFound, "not_found"},
		{ErrConflict, "conflict"},
		{ErrValidationFailed, "validation_failed"},
		{ErrInternalServer, "internal_server_error"},
		{ErrServiceUnavailable, "service_unavailable"},
		{ErrDatabaseError, "database_error"},
	}

	for _, test := range tests {
		if test.code != test.expectCode {
			t.Errorf("Error code mismatch: expected %s, got %s", test.expectCode, test.code)
		}
	}
}

// Helper functions
func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}

func newTestError(msg string) error {
	return &testError{msg}
}
