package sersan

import (
	"encoding/base32"
	"fmt"
	"reflect"
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
	Values map[interface{}]interface{}
	// When this session was created in UTC
	CreatedAt time.Time
	// When this session was last accessed in UTC
	AccessedAt time.Time
}

func NewSession(id, authId string, now time.Time) *Session {
	return &Session{
		ID:         id,
		AuthID:     authId,
		Values:     make(map[interface{}]interface{}),
		CreatedAt:  now,
		AccessedAt: now,
	}
}

// Note, we omit CreatedAt and AccessedAt
func (sess *Session) Equal(other *Session) bool {
	return sess.ID == other.ID && sess.AuthID == other.AuthID && reflect.DeepEqual(sess.Values, other.Values)
}

func (sess *Session) ExpireAt(IdleTimeout, absoluteTimeout int) time.Time {
	var (
		idle     time.Time
		absolute time.Time
	)

	if IdleTimeout != 0 {
		idle = sess.AccessedAt.Add(time.Second * time.Duration(IdleTimeout))
	}

	if absoluteTimeout != 0 {
		absolute = sess.CreatedAt.Add(time.Second * time.Duration(absoluteTimeout))
	}

	if idle.IsZero() {
		return absolute
	}

	if absolute.IsZero() {
		return idle
	}

	if idle.Before(absolute) {
		return idle
	}

	return absolute
}

func (sess *Session) MaxAge(IdleTimeout, absoluteTimeout int, now time.Time) int {
	expires := sess.ExpireAt(IdleTimeout, absoluteTimeout)

	if expires.IsZero() {
		return 0
	}
	if expires.Before(now) {
		return -1
	}

	return int(expires.Sub(now).Seconds())
}

func (sess *Session) IsSessionExpired(idleTimeout, absoluteTimeout int, now time.Time) bool {
	expires := sess.ExpireAt(idleTimeout, absoluteTimeout)

	if !expires.IsZero() && expires.After(now) {
		return false
	}
	return true
}

type DecomposedSession struct {
	AuthID     string
	Force      ForceInvalidate
	Decomposed map[interface{}]interface{}
}

func decomposeSession(authKey string, sess map[interface{}]interface{}) *DecomposedSession {
	var (
		authId = ""
		force  = DontForceInvalidate
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
		AuthID:     authId,
		Force:      force,
		Decomposed: sess,
	}
}

func recomposeSession(authKey, authId string, sess map[interface{}]interface{}) map[interface{}]interface{} {
	if authId != "" {
		sess[authKey] = authId
	}
	return sess
}

// The server-side session backend needs to maintain some statein order to work.
// This struct hold all info needed.
type ServerSessionState struct {
	// Cookie Name
	cookieName                   string
	AuthKey                      string
	storage                      Storage
	Options                      *Options
	Codecs                       []securecookie.Codec
	IdleTimeout, AbsoluteTimeout int
}

type SaveSessionToken struct {
	sess *Session
	now  time.Time
}

func NewServerSessionState(storage Storage, keyPairs ...[]byte) *ServerSessionState {
	return &ServerSessionState{
		cookieName:      "sersan:session",
		storage:         storage,
		Codecs:          securecookie.CodecsFromPairs(keyPairs...),
		IdleTimeout:     604800,  // 7 days
		AbsoluteTimeout: 5184000, // 60 days
		Options: &Options{
			Path:     "/",
			HttpOnly: true,
		},
	}
}

func (ss *ServerSessionState) SetCookieName(name string) error {
	if !isCookieNameValid(name) {
		return fmt.Errorf("sersan: invalid character in cookie name: %s", name)
	}
	ss.cookieName = name
	return nil
}

// Load the session map from the storage backend.
func (ss *ServerSessionState) Load(cookieValue string) (map[interface{}]interface{}, *SaveSessionToken, error) {
	var (
		err error
		now = time.Now().UTC()
	)

	if cookieValue != "" {
		sess, err := ss.storage.Get(cookieValue)
		if err == nil && sess != nil {
			if !sess.IsSessionExpired(ss.IdleTimeout, ss.AbsoluteTimeout, now) {
				return recomposeSession(ss.AuthKey, sess.AuthID, sess.Values), &SaveSessionToken{now: now, sess: sess}, err
			}
		}
	}

	data := make(map[interface{}]interface{})

	return data, &SaveSessionToken{now: now, sess: nil}, err
}

//
func (ss *ServerSessionState) Save(token *SaveSessionToken, data map[interface{}]interface{}) (*Session, error) {
	outputDecomp := decomposeSession(ss.AuthKey, data)
	sess, err := ss.invalidateIfNeeded(token.sess, outputDecomp)
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
		err    error
	)

	if sess != nil && sess.AuthID != "" {
		authID = sess.AuthID
	}

	invalidateCurrent := decomposed.Force != DontForceInvalidate || decomposed.AuthID != authID
	invalidateOthers := decomposed.Force == AllSessionIDsOfLoggedUser && decomposed.AuthID != ""

	if invalidateCurrent && sess != nil {
		err = ss.storage.Destroy(sess.ID)
		if err != nil {
			return nil, err
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
	nsess.Values = dec.Decomposed

	err = ss.storage.Replace(nsess)

	return nsess, err
}
