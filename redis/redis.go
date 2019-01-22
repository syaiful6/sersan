package redis

import (
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

// NewRediStore instantiates a RediStore with provided redis.Pool
func NewRediStore(pool *redis.Pool) (*RediStore, error) {
	rs := &RediStore{
		Pool:            pool,
		DefaultExpire:   86400 * 30, // 30 days
		IdleTimeout:     604800,     // 7 days
		AbsoluteTimeout: 5184000,    // 60 days
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

	return get(conn, rs.keyPrefix+id, rs.serializer)
}

func (rs *RediStore) Destroy(id string) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	sess, err := get(conn, rs.keyPrefix+id, rs.serializer)
	if err != nil {
		return err
	}
	// session didn't exists
	if sess == nil {
		return nil
	}

	if sess.AuthID != "" {
		_, err = destroyScript.Do(conn, rs.keyPrefix+id, rs.keyPrefix+sess.AuthID)
	} else {
		_, err = destroyScript.Do(conn, rs.keyPrefix+id, "")
	}

	return err
}

func (rs *RediStore) DestroyAllOfAuthId(authId string) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	_, err := destroyAllOfAuthIdScript.Do(conn, rs.keyPrefix+authId)
	return err
}

func (rs *RediStore) Insert(sess *sersan.Session) error {
	conn := rs.Pool.Get()
	defer conn.Close()

	if err := conn.Err(); err != nil {
		return err
	}

	oldSess, err := get(conn, rs.keyPrefix+sess.ID, rs.serializer)
	if err != nil {
		return err
	}
	if oldSess != nil {
		return &sersan.SessionAlreadyExists{OldSession: oldSess, NewSession: sess}
	}

	b, err := rs.serializer.Serialize(sess)
	if err != nil {
		return err
	}

	age := rs.getExpire(sess)

	authKey := ""
	if sess.AuthID != "" {
		authKey = rs.keyPrefix + sess.AuthID
	}

	_, err = insertScript.Do(conn, rs.keyPrefix+sess.ID, authKey, age, b)
	return err
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

	oldAuthKey := ""
	if oldSess.AuthID != "" {
		oldAuthKey = rs.keyPrefix + oldSess.AuthID
	}

	authKey := ""
	if sess.AuthID != "" {
		authKey = rs.keyPrefix + sess.AuthID
	}

	_, err = replaceScript.Do(conn, rs.keyPrefix+sess.ID, oldAuthKey, authKey, age, b)
	return err
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
	ss := sersan.NewServerSessionState(rs)
	ss.IdleTimeout = rs.IdleTimeout
	ss.AbsoluteTimeout = rs.AbsoluteTimeout

	age := ss.NextExpiresMaxAge(sess)
	if age <= 0 {
		age = rs.DefaultExpire
	}

	return age
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
