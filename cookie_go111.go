// +build go1.11

package sersan

import "net/http"

// newCookieFromOptions returns an http.Cookie with the options set.
func newCookieFromOptions(name, value string, maxAge int, options *Options) *http.Cookie {
	return &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     options.Path,
		Domain:   options.Domain,
		MaxAge:   maxAge,
		Secure:   options.Secure,
		HttpOnly: options.HttpOnly,
		SameSite: options.SameSite,
	}
}
