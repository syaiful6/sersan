package redis

import (
	"encoding/base32"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/gorilla/securecookie"

	"github.com/syaiful6/sersan"
)

const (
	defaultRedisHost = "127.0.0.1"
	defaultRedisPort = "6379"
)

func generateSessionId() string {
	return strings.TrimRight(
		base32.StdEncoding.EncodeToString(
			securecookie.GenerateRandomKey(32)), "=")
}

func generateSession(hasAuthID bool) *sersan.Session {
	id := generateSessionId()
	authId := ""
	if hasAuthID {
		authId = generateSessionId()
	}

	sess := sersan.NewSession(id, authId, time.Now().UTC())
	sess.AccessedAt = sess.AccessedAt.Add(time.Duration(rand.Uint32()) * time.Second)

	for i := 0; i < 20; i++ {
		sess.Values[strconv.Itoa(rand.Int())] = strconv.Itoa(rand.Int())
	}

	return sess
}

func cloneSession(sess *sersan.Session, authID string) *sersan.Session {
	nsess := sersan.NewSession(sess.ID, authID, sess.CreatedAt)
	nsess.AccessedAt = sess.AccessedAt

	for k, v := range sess.Values {
		nsess.Values[k] = v
	}

	return nsess
}

func dial(network, address string) (redis.Conn, error) {
	c, err := redis.Dial(network, address)
	if err != nil {
		return nil, err
	}
	return c, err
}

func createRedisPool() *redis.Pool {
	addr := os.Getenv("REDIS_HOST")
	if addr == "" {
		addr = defaultRedisHost
	}

	port := os.Getenv("REDIS_PORT")
	if port == "" {
		port = defaultRedisPort
	}

	return &redis.Pool{
		MaxIdle:     10,
		IdleTimeout: 240 * time.Second,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			return err
		},
		Dial: func() (redis.Conn, error) {
			return dial("tcp", fmt.Sprintf("%s:%s", addr, port))
		},
	}
}

func TestGetInsertDestroy(t *testing.T) {
	rs, err := NewRediStore(createRedisPool())
	if err != nil {
		t.Fatalf("can't create redistore, returned %v", err)
	}
	sess := generateSession(false)

	gsess, err := rs.Get(sess.ID)
	if err != nil || gsess != nil {
		t.Fatal("expected both sess and err return nil")
	}
	// now insert it
	err = rs.Insert(sess)
	if err != nil {
		t.Fatalf("Failed inserting session to redis. return %v", err)
	}

	gsess, err = rs.Get(sess.ID)
	if err != nil {
		t.Fatalf("Get returned error: %v", err)
	}
	assertSessionEqual(t, sess, gsess)

	err = rs.Destroy(sess.ID)
	if err != nil {
		t.Fatalf("Failed removing session from redis. return %v", err)
	}

	// now that session should be deleted
	gsess, err = rs.Get(sess.ID)
	if err != nil || gsess != nil {
		t.Fatal("expected both sess and err return nil")
	}
}

func TestNonExistDestroyAllOfAuthId(t *testing.T) {
	var nsess *sersan.Session
	var sess *sersan.Session

	rs, err := NewRediStore(createRedisPool())
	if err != nil {
		t.Fatalf("can't create redistore, returned %v", err)
	}
	err = rs.DestroyAllOfAuthId(generateSessionId())
	if err != nil {
		t.Fatalf("DestroyAllOfAuthId should not return error for non exists authId. %v", err)
	}

	// test if it only delete the relevant session
	master := generateSession(true)
	authID := master.AuthID

	preslaves := make([]*sersan.Session, 200)
	for i := 0; i < 200; i++ {
		preslaves[i] = generateSession(i > 100)
	}
	slaves := make([]*sersan.Session, 200)
	for i := 0; i < 200; i++ {
		slaves[i] = cloneSession(preslaves[i], authID)
	}

	others := make([]*sersan.Session, 60)
	for i := 0; i < 60; i++ {
		others[i] = generateSession(i > 30)
	}

	alls := append(slaves, master)
	alls = append(alls, others...)

	// Insert preslaves then replace them with slaves to
	// further test if the storage backend is able to maintain
	// its invariants regarding auth IDs.
	err = rs.Insert(master)
	if err != nil {
		t.Fatalf("Failed inserting session %s to redis. return %v", master.ID, err)
	}

	for _, sess = range preslaves {
		err = rs.Insert(sess)
		if err != nil {
			t.Fatalf("Failed inserting session %s to redis. return %v", sess.ID, err)
		}
	}
	for _, sess = range others {
		err = rs.Insert(sess)
		if err != nil {
			t.Fatalf("Failed inserting session %s to redis. return %v", sess.ID, err)
		}
	}
	for _, sess = range slaves {
		err = rs.Replace(sess)
		if err != nil {
			t.Fatalf("Failed Replace session %s to redis. return %v", sess.ID, err)
		}
	}

	for _, sess = range alls {
		nsess, err = rs.Get(sess.ID)
		if err != nil {
			t.Errorf("error getting session data for %s. error: %v", sess.ID, err)
		}

		if nsess == nil {
			t.Fatal("session should not nil if it exists")
		}

		assertSessionEqual(t, sess, nsess)
	}

	err = rs.DestroyAllOfAuthId(authID)
	if err != nil {
		t.Fatalf("DestroyAllOfAuthId returned error: %v", err)
	}

	// master and slaves should return nil
	sess, err = rs.Get(master.ID)
	if err != nil {
		t.Fatalf("error getting session data for %s. error: %v", master.ID, err)
	}
	if sess != nil {
		t.Fatal("session must deleted when it's AuthID deleted with DestroyAllOfAuthId")
	}

	for _, sess = range slaves {
		nsess, err = rs.Get(sess.ID)
		if err != nil {
			t.Fatalf("error getting session data for %s. error: %v", sess.ID, err)
		}
		if nsess != nil {
			t.Fatal("session must deleted when it's AuthID deleted with DestroyAllOfAuthId")
		}
	}

	for _, sess = range others {
		nsess, err = rs.Get(sess.ID)
		if err != nil {
			t.Fatalf("error getting session data for %s. error: %v", sess.ID, err)
		}
		if nsess == nil {
			t.Fatalf("session must not deleted when it's AuthID deleted with DestroyAllOfAuthId. AuthID: %v", nsess.AuthID)
		}
	}
}

func TestInsertThrowIfSessionExist(t *testing.T) {
	s1 := generateSession(true)
	s2 := generateSession(true)
	s2.ID = s1.ID

	rs, err := NewRediStore(createRedisPool())
	if err != nil {
		t.Fatalf("can't create redistore, returned %v", err)
	}

	sess, err := rs.Get(s1.ID)
	if err != nil || sess != nil {
		t.Fatal("expected both sess and err return nil")
	}

	err = rs.Insert(s1)
	if err != nil {
		t.Fatalf("Failed inserting session %s to redis. return %v", s1.ID, err)
	}

	err = rs.Insert(s2)
	if _, ok := err.(sersan.SessionAlreadyExists); !ok {
		t.Fatalf("Inserting existing session ID should return ersan.SessionAlreadyExists error. it return %v", err)
	}
}

func TestReplaceThrowIfSessionExist(t *testing.T) {
	s1 := generateSession(true)

	rs, err := NewRediStore(createRedisPool())
	if err != nil {
		t.Fatalf("can't create redistore, returned %v", err)
	}

	sess, err := rs.Get(s1.ID)
	if err != nil || sess != nil {
		t.Fatal("expected both sess and err return nil")
	}

	err = rs.Replace(s1)
	if _, ok := err.(sersan.SessionDoesNotExist); !ok {
		t.Fatalf("Replacing non existing session must return SessionDoesNotExist. it return %v", err)
	}
}

func assertSessionEqual(t *testing.T, a *sersan.Session, b *sersan.Session) {
	if !a.Equal(b) {
		t.Fatalf("session saved and get not equal. ID: %s == %s. AuthID: %s == %s, CreatedAt: %s == %s, AccessedAt: %s == %s. Values DeepEqual %v",
			a.ID, b.ID, a.AuthID, b.AuthID,
			a.CreatedAt.Format(time.UnixDate), b.CreatedAt.Format(time.UnixDate),
			a.AccessedAt.Format(time.UnixDate), b.AccessedAt.Format(time.UnixDate),
			reflect.DeepEqual(a.Values, b.Values))
	}
}
