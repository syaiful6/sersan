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
	sess.AccessedAt = sess.AccessedAt.Add(time.Duration(rand.Int()) * time.Second)

	for i := 0; i < 20; i++ {
		sess.Values[strconv.Itoa(rand.Int())] = strconv.Itoa(rand.Int())
	}

	return sess
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
	if !reflect.DeepEqual(sess, gsess) {
		t.Fatal("original session and saved session are different")
	}

	err = rs.Destroy(sess.ID)
	if err != nil {
		t.Fatalf("Failed removing session from redis")
	}

	// now that session should be deleted
	gsess, err = rs.Get(sess.ID)
	if err != nil || gsess != nil {
		t.Fatal("expected both sess and err return nil")
	}
}
