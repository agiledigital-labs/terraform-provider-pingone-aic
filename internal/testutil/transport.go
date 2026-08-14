// Package testutil holds helpers shared by tests in more than one package.
//
// It is only imported from _test.go files; nothing here ships in the provider
// binary.
package testutil

import "net/http"

// RoundTripFunc adapts a function to http.RoundTripper so a test can stand in
// for a tenant without opening a socket.
type RoundTripFunc func(*http.Request) (*http.Response, error)

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

// Client returns an *http.Client whose every request is served by f.
func Client(f RoundTripFunc) *http.Client {
	return &http.Client{Transport: f}
}
