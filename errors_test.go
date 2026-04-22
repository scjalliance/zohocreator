package zohocreator

import (
	"errors"
	"net/http"
	"testing"
)

func TestClassifyError(t *testing.T) {
	body := []byte(`{"code":2933,"message":"Permission denied"}`)
	cases := []struct {
		status int
		kind   string
		is     error
	}{
		{http.StatusBadRequest, "validation", ErrBadRequest},
		{http.StatusUnauthorized, "auth", ErrUnauthorized},
		{http.StatusForbidden, "forbidden", ErrForbidden},
		{http.StatusNotFound, "not_found", ErrNotFound},
		{http.StatusConflict, "conflict", ErrConflict},
		{http.StatusTooManyRequests, "rate_limited", ErrRateLimited},
		{http.StatusInternalServerError, "server", ErrServer},
		{http.StatusBadGateway, "server", ErrServer},
	}
	for _, c := range cases {
		err := classifyError(c.status, nil, body)
		if err == nil {
			t.Errorf("status=%d: nil error", c.status)
			continue
		}
		if !errors.Is(err, c.is) {
			t.Errorf("status=%d: !errors.Is %v (%T)", c.status, c.is, err)
		}
		var e interface{ Error() string }
		_ = errors.As(err, &e)
	}
}

func TestClassifyErrorRetryAfter(t *testing.T) {
	h := http.Header{}
	h.Set("Retry-After", "7")
	err := classifyError(http.StatusTooManyRequests, h, []byte(`{"code":2955}`))
	rle := &RateLimitError{}
	if !errors.As(err, &rle) {
		t.Fatalf("not RateLimitError: %T", err)
	}
	if rle.RetryAfter != 7 {
		t.Errorf("RetryAfter=%d", rle.RetryAfter)
	}
}

func TestClassifyErrorEmptyBody(t *testing.T) {
	err := classifyError(http.StatusNotFound, nil, nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestClassifyErrorDescriptionFallback(t *testing.T) {
	body := []byte(`{"code":1030,"description":"Authorization Failure. The access token is either invalid or has expired."}`)
	err := classifyError(http.StatusUnauthorized, nil, body)
	var ae *AuthError
	if !errors.As(err, &ae) {
		t.Fatalf("expected *AuthError, got %T", err)
	}
	if ae.Base.Message == "" || ae.Base.Code != 1030 {
		t.Errorf("expected description-derived message with code=1030, got base=%+v", ae.Base)
	}
}

func TestErrorMessages(t *testing.T) {
	cases := []error{
		&AuthError{Base: Error{Status: 401}},
		&AuthError{Base: Error{Status: 401, Message: "bad token"}},
		&ForbiddenError{Base: Error{Status: 403}},
		&NotFoundError{Base: Error{Status: 404}, ResourceType: "form", ResourceID: "X"},
		&ValidationError{Base: Error{Status: 400, Message: "oops"}},
		&RateLimitError{Base: Error{Status: 429, Message: "too fast"}},
		&ConflictError{Base: Error{Status: 409, Message: "dup"}},
		&APIError{Base: Error{Status: 500, Message: "boom"}},
	}
	for _, e := range cases {
		if e.Error() == "" {
			t.Errorf("%T: empty error message", e)
		}
	}
}
