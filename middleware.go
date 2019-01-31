package sersan

import (
	"context"
	"errors"
	"net/http"

	"github.com/gorilla/securecookie"
)

type sessionResponseWriter struct {
	http.ResponseWriter

	hasWritten bool

	data  map[interface{}]interface{}
	token *SaveSessionToken
	ss    *ServerSessionState
}

func newSessionResponseWriter(w http.ResponseWriter, token *SaveSessionToken) *sessionResponseWriter {
	return &sessionResponseWriter{
		ResponseWriter: w,
		token:          token,
	}
}

type sessionContextKey struct{}

// SessionMiddleware for loading and saving session data. Make sure to use this
// middleware.
func SessionMiddleware(ss *ServerSessionState) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sessId := ""
			if c, errCookie := r.Cookie(ss.cookieName); errCookie == nil {
				err := securecookie.DecodeMulti(ss.cookieName, c.Value, &sessId, ss.Codecs...)
				if err != nil {
					sessId = ""
				}
			}
			data, token, err := ss.Load(sessId)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			nw := newSessionResponseWriter(w, token)
			nw.data = data
			nw.ss = ss

			nr := r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, data))

			next.ServeHTTP(nw, nr)
		})
	}
}

// Get session data associated for this request. Make sure call this function after
// `SessionMiddleare` run.
func GetSession(r *http.Request) (map[interface{}]interface{}, error) {
	var ctx = r.Context()
	data := ctx.Value(sessionContextKey{})
	if data != nil {
		return data.(map[interface{}]interface{}), nil
	}

	return nil, errors.New("sersan: no session data found in request, perhaps you didn't use Sersan's middleware?")
}

func (w *sessionResponseWriter) WriteHeader(code int) {
	if !w.hasWritten {
		if err := w.saveSession(); err != nil {
			panic(err)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *sessionResponseWriter) Write(b []byte) (int, error) {
	if !w.hasWritten {
		if err := w.saveSession(); err != nil {
			return 0, err
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *sessionResponseWriter) saveSession() error {
	if w.hasWritten {
		panic("should not call saveSession twice")
	}

	w.hasWritten = true

	var (
		err  error
		sess *Session
	)

	if sess, err = w.ss.Save(w.token, w.data); err != nil {
		return err
	}

	if sess == nil {
		http.SetCookie(w,
			newCookieFromOptions(w.ss.cookieName, "", -1, w.ss.Options))
		return nil
	}

	encoded, err := securecookie.EncodeMulti(w.ss.cookieName, sess.ID,
		w.ss.Codecs...)
	if err != nil {
		return err
	}

	http.SetCookie(w,
		newCookieFromOptions(w.ss.cookieName, encoded,
			sess.MaxAge(w.ss.IdleTimeout, w.ss.AbsoluteTimeout, w.token.now), w.ss.Options))
	return nil
}
