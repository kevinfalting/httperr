package httperr

import (
	"net/http"
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
	return toStd(handler)
}
