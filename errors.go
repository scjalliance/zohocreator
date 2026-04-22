package zohocreator

import (
	"errors"
	"fmt"
)

// Sentinel errors exposed for errors.Is comparisons. Typed errors below wrap
// these so callers can also type-assert for extra detail.
var (
	ErrNotFound     = errors.New("resource not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrForbidden    = errors.New("forbidden")
	ErrBadRequest   = errors.New("bad request")
	ErrConflict     = errors.New("conflict")
	ErrRateLimited  = errors.New("rate limited")
	ErrServer       = errors.New("server error")
)

// Error is the base API error, carrying the HTTP status, Zoho's own error
// code, and the server message.
type Error struct {
	// Status is the HTTP status code.
	Status int
	// Code is Zoho Creator's own error code from the response body (e.g.
	// 3000 for success, 1030 for auth failure). Zero when the response was
	// not a standard JSON envelope.
	Code int
	// Message is the server-supplied error message.
	Message string
	// Kind is a short machine-readable category: auth, forbidden, not_found,
	// validation, rate_limited, server, api.
	Kind string
}

// Error satisfies the error interface.
func (e Error) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("zohocreator: %s (http=%d code=%d)", e.Message, e.Status, e.Code)
	}
	return fmt.Sprintf("zohocreator: %s (http=%d)", e.Message, e.Status)
}

// APIError is an uncategorised API error, returned for statuses we don't map
// to a more specific type.
type APIError struct{ Base Error }

// Error returns the underlying message.
func (e *APIError) Error() string { return e.Base.Error() }

// Unwrap returns ErrServer so errors.Is(err, ErrServer) works for generic
// 5xx-ish problems.
func (e *APIError) Unwrap() error { return ErrServer }

// AuthError represents a 401 response.
type AuthError struct{ Base Error }

// Error returns a message tailored to auth problems.
func (e *AuthError) Error() string {
	if e.Base.Message != "" {
		return "zohocreator: authentication failed - " + e.Base.Message
	}
	return "zohocreator: authentication failed - invalid or expired token"
}

// Unwrap returns ErrUnauthorized.
func (e *AuthError) Unwrap() error { return ErrUnauthorized }

// ForbiddenError represents a 403 response (authenticated but not permitted).
type ForbiddenError struct{ Base Error }

// Error returns a message tailored to permission problems.
func (e *ForbiddenError) Error() string {
	if e.Base.Message != "" {
		return "zohocreator: forbidden - " + e.Base.Message
	}
	return "zohocreator: forbidden - insufficient permissions"
}

// Unwrap returns ErrForbidden.
func (e *ForbiddenError) Unwrap() error { return ErrForbidden }

// NotFoundError represents a 404 response.
type NotFoundError struct {
	Base         Error
	ResourceType string // e.g. "form", "report", "record"
	ResourceID   string // id or link name of the missing resource
}

// Error returns a message including the resource, when known.
func (e *NotFoundError) Error() string {
	if e.ResourceType != "" && e.ResourceID != "" {
		return fmt.Sprintf("zohocreator: %s %q not found", e.ResourceType, e.ResourceID)
	}
	if e.Base.Message != "" {
		return "zohocreator: not found - " + e.Base.Message
	}
	return "zohocreator: resource not found"
}

// Unwrap returns ErrNotFound.
func (e *NotFoundError) Unwrap() error { return ErrNotFound }

// ValidationError represents a 400 response.
type ValidationError struct{ Base Error }

// Error returns a message tailored to validation failures.
func (e *ValidationError) Error() string {
	if e.Base.Message != "" {
		return "zohocreator: validation error - " + e.Base.Message
	}
	return "zohocreator: validation error"
}

// Unwrap returns ErrBadRequest.
func (e *ValidationError) Unwrap() error { return ErrBadRequest }

// ConflictError represents a 409 response (rare, but possible for some bulk ops).
type ConflictError struct{ Base Error }

// Error returns the underlying message.
func (e *ConflictError) Error() string {
	if e.Base.Message != "" {
		return "zohocreator: conflict - " + e.Base.Message
	}
	return "zohocreator: conflict"
}

// Unwrap returns ErrConflict.
func (e *ConflictError) Unwrap() error { return ErrConflict }

// RateLimitError represents a 429 response.
type RateLimitError struct {
	Base       Error
	RetryAfter int // seconds, from Retry-After header when present (0 when absent)
}

// Error returns a message tailored to rate-limit failures.
func (e *RateLimitError) Error() string {
	if e.Base.Message != "" {
		return "zohocreator: rate limited - " + e.Base.Message
	}
	return "zohocreator: rate limited"
}

// Unwrap returns ErrRateLimited.
func (e *RateLimitError) Unwrap() error { return ErrRateLimited }
