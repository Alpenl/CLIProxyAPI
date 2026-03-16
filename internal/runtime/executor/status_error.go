package executor

import (
	"net/http"
	"time"
)

type statusErr struct {
	code       int
	msg        string
	retryAfter *time.Duration
}

func (e statusErr) Error() string {
	return e.msg
}

func (e statusErr) StatusCode() int {
	return e.code
}

func (e statusErr) Headers() http.Header {
	return nil
}

func (e statusErr) RetryAfter() *time.Duration {
	if e.retryAfter == nil {
		return nil
	}
	out := *e.retryAfter
	return &out
}
