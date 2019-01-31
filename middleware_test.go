package sersan

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func newAppSetSession(key, value string, ss *ServerSessionState) http.Handler {
	return SessionMiddleware(ss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := GetSession(r)
		if err != nil {
			panic(err)
		}

		sess[key] = value

		w.Write([]byte(key + ":" + value))
	}))
}

func newAppGetSession(key string, ss *ServerSessionState) http.Handler {
	return SessionMiddleware(ss)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess, err := GetSession(r)
		if err != nil {
			panic(err)
		}

		if v, ok := sess[key]; ok {
			w.Write([]byte(v.(string)))
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		return
	}))
}

func TestBasicSession(t *testing.T) {
	var (
		r       *http.Request
		w       *httptest.ResponseRecorder
		hdr     http.Header
		cookies []string
		ok      bool
		body    string
	)
	// Round 1: set session key "foo" with value "bar"
	storage := NewStorageRecorder()
	ss := NewServerSessionState(storage, []byte("secret-key"))
	ss.SetCookieName("session-name")
	handler := newAppSetSession("foo", "bar", ss)

	r = httptest.NewRequest("GET", "http://localhost:8080/", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	hdr = w.Header()
	cookies, ok = hdr["Set-Cookie"]
	if !ok || len(cookies) != 1 {
		t.Fatal("No cookies. Header:", hdr)
	}

	// Round 2: get session key "foo", expect body written == "bar"
	handler = newAppGetSession("foo", ss)
	r = httptest.NewRequest("GET", "http://localhost:8080/", nil)
	r.Header.Add("Cookie", cookies[0])
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, r)
	if body = w.Body.String(); body != "bar" {
		t.Fatalf("session values not persisted correctly, want 'bar', actual '%s'", body)
	}
}
