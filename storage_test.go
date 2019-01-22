package sersan

import (
	"encoding/base32"
	"math/rand"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/securecookie"
)

func generateSessionId() string {
	return strings.TrimRight(
		base32.StdEncoding.EncodeToString(
			securecookie.GenerateRandomKey(32)), "=")
}

func generateSession(hasAuthID bool) *Session {
	id := generateSessionId()
	authId := ""
	if hasAuthID {
		authId = generateSessionId()
	}

	sess := NewSession(id, authId, time.Now().UTC())
	sess.AccessedAt = sess.AccessedAt.Add(time.Duration(rand.Int()) * time.Second)

	for i := 0; i < 20; i++ {
		sess.Values[strconv.Itoa(rand.Int())] = strconv.Itoa(rand.Int())
	}

	return sess
}

func TestGet(t *testing.T) {
	sess1 := NewSession("123456789-123456789-123456789-12", "foo", time.Now().UTC())
	tests := []struct {
		sess         []*Session
		getId        string
		expectedSess *Session
	}{
		{[]*Session{}, generateSessionId(), nil},
		{[]*Session{sess1}, "123456789-123456789-123456789-12", sess1},
	}

	var (
		storage *StorageRecorder
		s       *Session
	)

	for i, test := range tests {
		storage = PrepareStorageRecorder(test.sess)
		s, _ = storage.Get(test.getId)
		if test.expectedSess != nil && !reflect.DeepEqual(s, test.expectedSess) {
			t.Errorf("%d: Expected session returned with ID %s, return %s instead", i, test.expectedSess.ID, s.ID)
		}
	}
}

func TestDestroyNotExists(t *testing.T) {
	storage := NewStorageRecorder()
	id := generateSessionId()
	err := storage.Destroy(id)
	if err != nil {
		t.Errorf("storage.Destroy should not return error if it doesn't exist. %v", err)
	}
}

func TestGetDestroyInsertOp(t *testing.T) {
	storage := NewStorageRecorder()
	s := generateSession(true)

	var (
		s1  *Session
		err error
	)

	s1, err = storage.Get(s.ID)
	if err != nil {
		t.Errorf("storage.Get should not return error if it doesn't exist. %v", err)
	}
	if s1 != nil {
		t.Errorf("Expected nil returned in empty storage. return session ID %s instead", s.ID)
	}

	err = storage.Insert(s)
	if err != nil {
		t.Errorf("Storage insert should not return error. return %v instead.", err)
	}

	s1, err = storage.Get(s.ID)
	if !reflect.DeepEqual(s, s1) {
		t.Errorf("inserted session (%s) != get session (%s).", s.ID, s1.ID)
	}

	// now destroy it
	err = storage.Destroy(s.ID)
	if err != nil {
		t.Errorf("Expected non nil error. return error %v instead", err)
	}

	s1, err = storage.Get(s.ID)
	if err != nil {
		t.Errorf("storage.Get should not return error if it doesn't exist. %v", err)
	}
	if s1 != nil {
		t.Errorf("Expected nil returned in empty storage. return session ID %s instead", s.ID)
	}
}
