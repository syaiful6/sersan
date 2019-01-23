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

	_, err = destroyScript.Do(conn, rs.keyPrefix+id, rs.authKey(sess.AuthID))

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

	_, err = insertScript.Do(
		conn, rs.keyPrefix+sess.ID, rs.authKey(sess.AuthID), rs.getExpire(sess), b)
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
