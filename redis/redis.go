package redis

import (
	"time"

	"github.com/gomodule/redigo/redis"

	"github.com/syaiful6/sersan"
)

// 30 days
const defaultSessionExpire = 86400 * 30

type RediStore struct {
	Pool                         *redis.Pool
	DefaultExpire                int
	keyPrefix                    string
	serializer                   SessionSerializer
	IdleTimeout, AbsoluteTimeout int
}

func (rs *RediStore) SetKeyPrefix(p string) {
	rs.keyPrefix = p
}

func (rs *RediStore) SetDefaultExpire(age int) {
	rs.DefaultExpire = age
}

// Copy of Session field, except value to be used in "HMSET" and "HMGETALL"
type SessionHash struct {
	// Value of authentication ID, separate from rest
	AuthID string
	// Values contains the user-data for the session.
	Values []byte
	// When this session was created in UTC
	CreatedAt string
	// When this session was last accessed in UTC
	AccessedAt string
}

func newSessionHashFrom(sess *sersan.Session, serializer SessionSerializer) (*SessionHash, error) {
	var sh = new(SessionHash)

	sh.AuthID = sess.AuthID
	sh.CreatedAt = sess.CreatedAt.Format(time.UnixDate)
	sh.AccessedAt = sess.AccessedAt.Format(time.UnixDate)

	bytes, err := serializer.Serialize(sess)
	if err != nil {
		return nil, err
	}

	sh.Values = bytes
	return sh, nil
}

func (sh *SessionHash) toSession(id string, serializer SessionSerializer) (*sersan.Session, error) {
	createdAt, err := time.Parse(time.UnixDate, sh.CreatedAt)
	if err != nil {
		return nil, err
	}

	sess := sersan.NewSession(id, sh.AuthID, createdAt)

	accessedAt, err := time.Parse(time.UnixDate, sh.AccessedAt)
	if err != nil {
		return nil, err
	}
	sess.AccessedAt = accessedAt

	err = serializer.Deserialize(sh.Values, sess)
	if err != nil {
		return nil, err
	}

	sess.ID = id
	sess.AuthID = sh.AuthID

	return sess, nil
}

// NewRediStore instantiates a RediStore with provided redis.Pool
func NewRediStore(pool *redis.Pool) (*RediStore, error) {
	rs := &RediStore{
		Pool:            pool,
		DefaultExpire:   604800,
		IdleTimeout:     604800,  // 7 days
		AbsoluteTimeout: 5184000, // 60 days
		keyPrefix:       "sersan:redis:",
		serializer:      GobSerializer{},
	}
	_, err := rs.ping()
	return rs, err
}

func (rs *RediStore) Get(id string) (*sersan.Session, error) {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return nil, err
	}

	data, err := redis.Values(conn.Do("HGETALL", rs.keyPrefix+id))
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var sh = new(SessionHash)
	if err = redis.ScanStruct(data, sh); err != nil {
		return nil, err
	}

	return sh.toSession(id, rs.serializer)
}

func (rs *RediStore) Destroy(id string) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	sk := rs.keyPrefix + id
	authID, err := redis.String(conn.Do("HGET", sk, "AuthID"))
	if err != nil {
		if err == redis.ErrNil {
			return nil
		}
		return err
	}

	conn.Send("MULTI")
	err = conn.Send("DEL", sk)
	if authID != "" {
		conn.Send("SREM", rs.authKey(authID), sk)
	}
	_, err = conn.Do("EXEC")
	return err
}

func (rs *RediStore) DestroyAllOfAuthId(authId string) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	authKey := rs.authKey(authId)
	sessionIDs, err := redis.Strings(conn.Do("SMEMBERS", authKey))
	if err != nil {
		return err
	}
	_, err = conn.Do("DEL", redis.Args{}.Add(authKey).AddFlat(sessionIDs)...)

	return err
}

func (rs *RediStore) Insert(sess *sersan.Session) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	sk := rs.keyPrefix + sess.ID
	exist, err := redis.Bool(conn.Do("EXISTS", sk, "AuthID"))
	if err != nil {
		return err
	}
	if exist {
		return sersan.SessionAlreadyExists{ID: sess.ID}
	}

	sh, err := newSessionHashFrom(sess, rs.serializer)
	if err != nil {
		return err
	}

	args := redis.Args{}.Add(sk).Add(rs.authKey(sess.AuthID))
	args = args.Add(rs.getExpire(sess)).AddFlat(sh)
	_, err = insertScript.Do(conn, args...)
	if err != nil {
		return err
	}
	return nil
}

func (rs *RediStore) Replace(sess *sersan.Session) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	sk := rs.keyPrefix + sess.ID
	oldAuthID, err := redis.String(conn.Do("HGET", sk, "AuthID"))
	if err != nil {
		if err == redis.ErrNil {
			return sersan.SessionDoesNotExist{ID: sess.ID}
		}
		return err
	}

	sh, err := newSessionHashFrom(sess, rs.serializer)
	if err != nil {
		return err
	}
	args := redis.Args{}.Add(sk).Add(rs.authKey(sess.AuthID))
	args = args.Add(rs.authKey(oldAuthID)).Add(rs.getExpire(sess)).AddFlat(sh)
	_, err = replaceScript.Do(conn, args...)
	if err != nil {
		return err
	}

	return nil
}

func (rs *RediStore) authKey(authId string) string {
	if authId != "" {
		return rs.keyPrefix + ":auth:" + authId
	}
	return ""
}

func (rs *RediStore) ping() (bool, error) {
	conn := rs.Pool.Get()
	defer conn.Close()
	data, err := conn.Do("PING")
	if err != nil || data == nil {
		return false, err
	}
	return (data == "PONG"), nil
}

func (rs *RediStore) getExpire(sess *sersan.Session) int {
	expire := sess.MaxAge(rs.IdleTimeout, rs.AbsoluteTimeout, time.Now().UTC())
	if expire <= 0 {
		return rs.DefaultExpire
	}
	return expire
}
