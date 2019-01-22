// +build go1.11

package sersan

import (
	"net/http"
	"testing"
)

// Test for setting SameSite field in new http.Cookie from name, value
// and options
func TestNewCookieFromOptionsSameSite(t *testing.T) {
	tests := []struct {
		sameSite http.SameSite
	}{
		{http.SameSiteDefaultMode},
		{http.SameSiteLaxMode},
		{http.SameSiteStrictMode},
	}
	for i, v := range tests {
		options := &Options{
			SameSite: v.sameSite,
		}
		cookie := newCookieFromOptions("", "", 0, options)
		if cookie.SameSite != v.sameSite {
			t.Fatalf("%v: bad cookie sameSite: got %v, want %v", i+1, cookie.SameSite, v.sameSite)
		}
	}
}