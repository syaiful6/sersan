package sersan

import (
	"context"
	"errors"
	"net/http"

	"github.com/gorilla/securecookie"
)

type SessionResponseWriter struct {
	http.ResponseWriter

	hasWritten bool

	data        map[interface{}]interface{}
	token       *SaveSessionToken
	serverState *ServerSessionState
}

func newSessionResponseWriter(w http.ResponseWriter, token *SaveSessionToken) *SessionResponseWriter {
	return &SessionResponseWriter{
		ResponseWriter: w,
		token:          token,
	}
}

type sessionContextKey struct{}

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
			nw.serverState = ss

			nr := r.WithContext(context.WithValue(r.Context(), sessionContextKey{}, data))

			next.ServeHTTP(nw, nr)
		})
	}
}

func GetSession(r *http.Request) (map[interface{}]interface{}, error) {
	var ctx = r.Context()
	data := ctx.Value(sessionContextKey{})
	if data != nil {
		return data.(map[interface{}]interface{}), nil
	}

	return nil, errors.New("sersan: no session data found in request, perhaps you didn't use Sersan's middleware?")
}

func (w *SessionResponseWriter) WriteHeader(code int) {
	if !w.hasWritten {
		if err := w.saveSession(); err != nil {
			panic(err)
		}
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *SessionResponseWriter) Write(b []byte) (int, error) {
	if !w.hasWritten {
		if err := w.saveSession(); err != nil {
			return 0, err
		}
	}
	return w.ResponseWriter.Write(b)
}

func (w *SessionResponseWriter) saveSession() error {
	if w.hasWritten {
		panic("should not call saveSession twice")
	}

	var (
		err  error
		sess *Session
	)

	if sess, err = w.serverState.Save(w.token, w.data); err != nil {
		return err
	}

	if sess == nil {
		http.SetCookie(w, newCookieFromOptions(w.serverState.cookieName, "", -1, w.serverState.Options))
		return nil
	}

	encoded, err := securecookie.EncodeMulti(w.serverState.cookieName, sess.ID,
		w.serverState.Codecs...)
	if err != nil {
		return err
	}

	http.SetCookie(w, newCookieFromOptions(w.serverState.cookieName, encoded, w.serverState.nextExpiresMaxAge(sess), w.serverState.Options))
	return nil
}
