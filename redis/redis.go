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

func newSessionHashFrom(sess *sersan.Session, serializer SessionSerializer) (*sersan.Session, error) {
	var sh *SessionHash

	sh.AuthID = sess.AuthID
	sh.CreatedAt = time.Format(time.UnixDate)
	sh.AccessedAt = time.Format(time.UnixDate)

	bytes, err := serializer.Serialize(sess)
	if err != nil {
		return nil, err
	}

	sh.Values = bytes
	return sh
}

func (sh *SessionHash) toSession(id string, serializer SessionSerializer) (*sersan.Session, error) {
	var sess *sersan.Session
	createdAt, err := time.Parse(time.UnixDate, sh.CreatedAt)
	if err != nil {
		return nil, err
	}
	sess.CreatedAt = createdAt

	accessedAt, err := time.Parse(time.UnixDate, sh.AccessedAt)
	if err != nil {
		return nil, err
	}
	sess.AccessedAt = accessedAt

	values, err = serializer.Deserialize(sh.Values, sess)
	if err != nil {
		return err
	}

	sess.ID = id
	sess.AuthID = sh.AuthID

	return sess, nil
}

// NewRediStore instantiates a RediStore with provided redis.Pool
func NewRediStore(pool *redis.Pool) (*RediStore, error) {
	rs := &RediStore{
		Pool:            pool,
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

	data, err := conn.Do("HGETALL", rs.keyPrefix+id)
	if err != nil {
		return nil, err
	}

	if len(data) == 0 {
		return nil, nil
	}

	var sh *SessionHash
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

	_, err := destroyScript.Do(conn, rs.keyPrefix+id, rs.keyPrefix + ":auth:")
	return err
}

func (rs *RediStore) DestroyAllOfAuthId(authId string) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	_, err := destroyAllOfAuthIdScript.Do(conn, rs.authKey(authId))
	return err
}

func (rs *RediStore) Insert(sess *sersan.Session) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	sh, err := newSessionHashFrom(sess, rs.serializer)
	if err != nil {
		return err
	}

	args := redis.Args{}.Add(rs.keyPrefix+sess).Add(rs.authKey(sess.AuthID)).Add(rs.getExpire(sess)).AddFlat(sh)
	reply, err = insertScript.Do(args...)
	if err != nil {
		return err
	}

}

func (rs *RediStore) Replace(sess *sersan.Session) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	oldSess, err := get(conn, rs.keyPrefix+sess.ID, rs.serializer)
	if err != nil {
		return err
	}
	if oldSess == nil {
		return &sersan.SessionDoesNotExist{Session: sess}
	}

	b, err := rs.serializer.Serialize(sess)
	if err != nil {
		return err
	}

	age := rs.getExpire(sess)

	_, err = replaceScript.Do(conn, rs.keyPrefix+sess.ID, rs.authKey(oldSess.AuthID),
		rs.authKey(sess.AuthID), age, b)
	return err
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
	return sess.MaxAge(rs.IdleTimeout, rs.AbsoluteTimeout, time.Now().UTC())
}

func get(c redis.Conn, key string, serializer SessionSerializer) (*sersan.Session, error) {
	data, err := c.Do("GET", key)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil // no data was associated with this key
	}

	b, err := redis.Bytes(data, err)
	if err != nil {
		return nil, err
	}
	sess := new(sersan.Session)
	err = serializer.Deserialize(b, sess)

	return sess, err
}
