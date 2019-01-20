package sersan

import (
	"encoding/base32"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/securecookie"
)

type ForceInvalidate int

const (
	CurrentSessionID ForceInvalidate = iota + 1
	_
	AllSessionIDsOfLoggedUser
	DontForceInvalidate
)

const (
	ForceInvalidateKey = "_forceinvalidate"
)

// Representation of a saved session
type Session struct {
	// session's id, primary key
	ID string
	// Value of authentication ID, separate from rest
	AuthID string
	// Values contains the user-data for the session.
	Values  map[interface{}]interface{}
	// When this session was created in UTC
	CreatedAt time.Time
	// When this session was last accessed in UTC
	AccessedAt time.Time
}

func NewSession(id, authId string, now) *Session {
	return &Session{
		ID: id,
		AuthID: authId,
		Values: make(map[interface{}]interface{}),
		CreatedAt: now,
		AccessedAt: now,
	}
}

type DecomposedSession struct {
	AuthID string
	Force ForceInvalidate
	Decomposed map[interface{}]interface{}
}

func decomposeSession(authKey string, sess map[interface{}]interface{}) *DecomposedSession {
	var (
		authId = ""
		force = DontForceInvalidate
	)
	if v, ok := sess[authKey]; ok {
		delete(sess, authKey)
		authId = v.(string)
	}
	if v, ok := sess[ForceInvalidateKey]; ok {
		delete(sess, ForceInvalidateKey)
		force = v.(ForceInvalidate)
	}

	return &DecomposedSession{
		AuthID: authId,
		Force: force,
		Decomposed: sess,
	}
}

func recomposeSession(authKey, authId string, sess map[interface{}]interface{}) map[interface{}]interface{} {
	if authId != "" {
		sess[authKey] = authId
	}
	return sess
}

// A storage backend, for server-side sessions.
type Storage interface {
	// Get the session for the given session ID. Returns nil if it not exists
	// rather than returning error
	Get(id string) (*Session, error)
	// Delete the session with given session ID. Does not do anything if the session
	// is not found.
	Destroy(id string) error
	// Delete all sessions of the given auth ID. Does not do anything if there
	// are no sessions of the given auth ID.
	DestroyAllOfAuthId(authId string) error
	// Insert a new session. return 'SessionAlreadyExists' error there already
	// exists a session with the same session ID. We only call this method after
	// generating a fresh session ID
	Insert(sess *Session) error
	// Replace the contents of a session. Return 'SessionDoesNotExist' if
	// there is no session with the given  session ID
	Replace(sess *Session) error
}

// The server-side session backend needs to maintain some statein order to work.
// This struct hold all info needed.
type ServerSessionState struct {
	// Cookie Name
	CookieName string
	AuthKey string
	storage Storage
	Options *Options
	Codecs []securecookie.Codec	
	IdleTimeout, AbsoluteTimeout int
}

type SaveSessionToken struct {
	sess *Session
	now time.Time
}

func NewServerSessionState(storage Storage, codecs []securecookie.Codec) *ServerSessionState {
	return &ServerSessionState{
		CookieName: "sersan:session",
		storage: storage,
		Codecs: codecs,
		Options: &Options{
			Path: "/",
			HttpOnly: true,
		}
	}
}

func (ss *ServerSessionState) NextExpires(session *Session) time.Time {
	var (
		idle time.Time
		absolute time.Time
	)

	if ss.IdleTimeout != 0 {
		idle = session.AccessedAt.Add(time.Second * ss.IdleTimeout)
	}

	if ss.AbsoluteTimeout != 0 {
		absolute = session.CreatedAt.Add(time.Second * ss.AbsoluteTimeout)
	}

	if idle.Before(absolute) {
		return idle
	}

	return absolute
}

func (ss *ServerSessionState) IsSessionExpired(now time.Time, session *Session) bool {
	expires := ss.NextExpires(sess)
	if expires.After(now) {
		return true
	}
	return false
}

// Load the session map from the storage backend.
func (ss *ServerSessionState) Load(r *http.Request) (map[interface{}]interface{}, *SaveSessionToken, error) {
	var (
		err error
		now = time.Now().UTC()
	)
	if c, errCookie := r.Cookie(ss.CookieName); errCookie == nil {
		sessId := ""
		err = securecookie.DecodeMulti(ss.CookieName, c.Value, &sessId, ss.CookieName.Codecs...)
		if err == nil {
			sess, err := ss.storage.Get(sessId)
			if err != nil && sess != nil {
				if !ss.IsSessionExpired(now, sess) {
					return recomposeSession(ss.AuthKey, sess.AuthID, sess.Values), &SaveSessionToken{now: now, sess: sess,}, err
				}
			}
		}
	}

	data := make(map[interface{}]interface{})

	return data, &SaveSessionToken{now: now, sess: data}, err
}

// 
func (ss *ServerSessionState) Save(token *SaveSessionToken, data map[interface{}]interface{}) (*Session, error) {
	outputDecomp := decomposeSession(ss.AuthKey, data)
	sess, err := ss.invalidateIfNeeded(ss.sess, outputDecomp)
	if err != nil {
		return nil, err
	}

	return ss.saveSessionOnDb(token.now, sess, outputDecomp)
}

// Invalidates an old session ID if needed. Returns the 'Session' that should be
// replaced when saving the session, if any.
//
// Currently we invalidate whenever the auth ID has changed (login, logout, different user)
// in order to prevent session fixation attacks.  We also invalidate when asked to via
// `forceInvalidate`
func (ss *ServerSessionState) invalidateIfNeeded(sess *Session, decomposed *DecomposedSession) (*Session, error) {
	var (
		authID string
		err error
	)

	if sess != nil && sess.AuthID != "" {
		authID = sess.AuthID
	}

	invalidateCurrent := decomposed.force != DontForceInvalidate || decomposed.AuthID != authID
	invalidateOthers := decomposed.force == AllSessionIDsOfLoggedUser && decomposed.AuthID != ""

	if invalidateCurrent && sess != nil {
		err = ss.storage.Destroy(sess.ID)
		if err != nil {
			nil, err
		}
	}

	if invalidateOthers && sess != nil {
		err = ss.storage.DestroyAllOfAuthId(sess.AuthID)
		if err != nil {
			return nil, err
		}
	}

	if invalidateCurrent {
		return nil, err
	}

	return sess, err
}

func (ss *ServerSessionState) saveSessionOnDb(now time.Time, sess *Session, dec *DecomposedSession) (*Session, error) {
	var err error
	
	if sess == nil && dec.AuthID == "" && len(dec.Decomposed) == 0 {
		return nil, err
	}

	if sess == nil {
		id := strings.TrimRight(
			base32.StdEncoding.EncodeToString(
				securecookie.GenerateRandomKey(32)), "=")
		sess = NewSession(id, dec.AuthID, now)
		sess.Values = dec.Decomposed

		err = ss.storage.Insert(sess)

		return sess, err
	}

	nsess := NewSession(sess.ID, dec.AuthID, now)
	nsess.CreatedAt = sess.CreatedAt
	nsess.Values = sess.Values

	err = ss.storage.Replace(sess)

	return nsess, err
}