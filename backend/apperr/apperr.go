// Package apperr defines the application error type mapped to the API error contract.
package apperr

import (
	"errors"
	"net/http"
)

// Error codes from the API contract.
const (
	CodeValidation         = "validation_error"
	CodeInvalidCredentials = "invalid_credentials"
	CodeUnauthorized       = "unauthorized"
	CodeInvalidToken       = "invalid_token"
	CodeTokenExpired       = "token_expired"
	CodeEmailNotVerified   = "email_not_verified"
	CodeAccountDisabled    = "account_disabled"
	CodeForbidden          = "forbidden"
	CodeInvalidMFACode     = "invalid_mfa_code"
	CodeNotFound           = "not_found"
	CodeConflict           = "conflict"
	CodeRateLimited        = "rate_limited"
	CodeInternal           = "internal_error"
)

// Error is an error carrying an HTTP status and a contract error code.
type Error struct {
	Status  int
	Code    string
	Message string
	Details map[string]string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

// New builds an Error.
func New(status int, code, msg string) *Error {
	return &Error{Status: status, Code: code, Message: msg}
}

// Validation builds a 422 validation_error with per-field details.
func Validation(details map[string]string) *Error {
	return &Error{Status: http.StatusUnprocessableEntity, Code: CodeValidation, Message: "Validation failed", Details: details}
}

func InvalidCredentials() *Error {
	return New(http.StatusUnauthorized, CodeInvalidCredentials, "Invalid email or password")
}
func Unauthorized(msg string) *Error { return New(http.StatusUnauthorized, CodeUnauthorized, msg) }
func InvalidToken(msg string) *Error { return New(http.StatusUnauthorized, CodeInvalidToken, msg) }
func TokenExpired() *Error           { return New(http.StatusUnauthorized, CodeTokenExpired, "Token expired") }
func EmailNotVerified() *Error {
	return New(http.StatusForbidden, CodeEmailNotVerified, "Email address is not verified")
}
func AccountDisabled() *Error {
	return New(http.StatusForbidden, CodeAccountDisabled, "Account is disabled")
}
func Forbidden(msg string) *Error { return New(http.StatusForbidden, CodeForbidden, msg) }
func InvalidMFACode() *Error {
	return New(http.StatusUnauthorized, CodeInvalidMFACode, "Invalid verification code")
}
func NotFound(msg string) *Error { return New(http.StatusNotFound, CodeNotFound, msg) }
func Conflict(msg string) *Error { return New(http.StatusConflict, CodeConflict, msg) }
func RateLimited() *Error {
	return New(http.StatusTooManyRequests, CodeRateLimited, "Too many requests, slow down")
}
func Internal() *Error {
	return New(http.StatusInternalServerError, CodeInternal, "Internal server error")
}

// As converts any error into an *Error (unknown errors become internal_error).
func As(err error) *Error {
	var e *Error
	if errors.As(err, &e) {
		return e
	}
	return Internal()
}
