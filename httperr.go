package httperr

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Handler responds to an http request and can return an error.
type Handler interface {
	ServeHTTP(http.ResponseWriter, *http.Request) error
}

// HandlerFunc is a function type that satisfies the [Handler] interface.
type HandlerFunc func(http.ResponseWriter, *http.Request) error

// ServeHTTP satisfies the [Handler] interface.
func (h HandlerFunc) ServeHTTP(w http.ResponseWriter, r *http.Request) error {
	return h(w, r)
}

// Middleware is a function type for wrapping [Handler] types.
type Middleware func(Handler) Handler

// WrapCommon wraps a common set of [Middleware] around a specific set of
// [Middleware] and a [HandlerFunc]. The first [Middlware] provided is the first
// invoked on a request.
func WrapCommon(common ...Middleware) func(HandlerFunc, ...Middleware) Handler {
	return func(h HandlerFunc, specific ...Middleware) Handler {
		handler := Wrap(h, specific...)
		handler = Wrap(handler.ServeHTTP, common...)

		return HandlerFunc(func(w http.ResponseWriter, r *http.Request) error {
			return handler.ServeHTTP(w, r)
		})
	}
}

// Wrap will wrap a set of [Middleware] around a [HandlerFunc]. The first
// [Middleware] provided is the first invoked on a request.
func Wrap(h HandlerFunc, mw ...Middleware) Handler {
	if len(mw) == 0 {
		return h
	}

	return mw[0](Wrap(h, mw[1:]...))
}

// ToStd is a function type for converting a [Handler] to an [http.Handler].
type ToStd func(Handler) http.Handler

// WrapCommonToStd wraps a common set of [Middleware] around a specific set of
// [Middleware] and a [HandlerFunc] in a way that is compatible with the stdlib
// [http.Handler].
func WrapCommonToStd(toStd ToStd, common ...Middleware) func(HandlerFunc, ...Middleware) http.Handler {
	wrap := WrapCommon(common...)
	return func(h HandlerFunc, specific ...Middleware) http.Handler {
		return toStd(wrap(h, specific...))
	}
}

// WrapToStd wraps a set of [Middlware] around a [HandlerFunc] in a way that is
// compatible with the stdlib [http.Handler].
func WrapToStd(h HandlerFunc, toStd ToStd, mw ...Middleware) http.Handler {
	handler := Wrap(h, mw...)
	return HandleErr(nil, nil)(handler)
}

// ErrFunc defines the function signature required to handle error responses.
// Modeled from the [http.Error] function.
type ErrFunc func(w http.ResponseWriter, err string, code int)

// HandleErr returns a [ToStd] by providing a way to handle all errors before
// passing back to a [http.Handler] for compatability with the stdlib.
func HandleErr(errWriter io.Writer, errFunc ErrFunc) ToStd {
	if errWriter == nil {
		errWriter = os.Stderr
	}

	if errFunc == nil {
		errFunc = http.Error
	}

	return func(h Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			err := h.ServeHTTP(w, r)
			if err == nil {
				return
			}

			var e interface{ StatusMsg() (int, string) }
			if errors.As(err, &e) {
				status, msg := e.StatusMsg()
				errFunc(w, msg, status)
			} else {
				errFunc(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}

			fmt.Fprint(errWriter, err)
		})
	}
}

type handlerError struct {
	err         error
	status      int
	responseMsg string
}

// StatusMsg will return the http status code and message to return to the
// client.
func (h *handlerError) StatusMsg() (int, string) {
	return h.status, h.responseMsg
}

// Error satisfies the error interface
func (h *handlerError) Error() string {
	return fmt.Sprintf("status=%d msg=%q err=%q", h.status, h.responseMsg, h.err)
}

// Is reports whether any error in err's tree matches target.
func (h *handlerError) Is(target error) bool {
	return errors.Is(h.err, target)
}

// As finds the first error in err's tree that matches target, and if one is
// found, sets target to that error value and returns true. Otherwise, it
// returns false.
func (h *handlerError) As(target any) bool {
	return errors.As(h.err, target)
}

// Unwrap returns the underlying error
func (h *handlerError) Unwrap() error {
	return h.err
}

// NewError will return an error that can be used by the ErrorHandler. The error
// itself is not sent back to the client, but logged instead. The status and
// optional responseMsg(s) are both used to respond to the client.
func NewError(err error, status int, responseMsg ...string) error {
	return &handlerError{
		err:         err,
		status:      status,
		responseMsg: strings.Join(responseMsg, " "),
	}
}
