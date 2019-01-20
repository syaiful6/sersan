// +build !go1.11

package sersan

// Options stores configuration for a session or session store.
//
// Fields are a subset of http.Cookie fields.
type Options struct {
	Path   string
	Domain string
	Secure   bool
	HttpOnly bool
}