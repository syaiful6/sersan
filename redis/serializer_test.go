package redis

import (
	"encoding/gob"

	"reflect"
	"testing"
	"time"

	"github.com/syaiful6/sersan"
)

func TestJSONSerializer(t *testing.T) {
	serializer := JSONSerializer{}
	sess := sersan.NewSession("foo", "john", time.Now().UTC())
	sess.Values["bar"] = "baz"
	bytes, err := serializer.Serialize(sess)
	if err != nil {
		t.Fatalf("JSONSerializer.Serialize expected non nil error, it return error: %v", err)
	}

	sess2 := new(sersan.Session)
	err = serializer.Deserialize(bytes, sess2)
	if err != nil {
		t.Fatalf("JSONSerializer.Deserialize expected non nil error, it return error: %v", err)
	}

	if !reflect.DeepEqual(sess, sess2) {
		t.Fatal("JSONSerializer serialize(deserialize(sess)) != sess")
	}
}

type testKey struct{}

func TestGobSerializer(t *testing.T) {
	serializer := GobSerializer{}
	sess := sersan.NewSession("foo", "john", time.Now().UTC())
	sess.Values[testKey{}] = "baz"
	bytes, err := serializer.Serialize(sess)
	if err != nil {
		t.Fatalf("GobSerializer.Serialize expected non nil error, it return error: %v", err)
	}

	sess2 := new(sersan.Session)
	err = serializer.Deserialize(bytes, sess2)
	if err != nil {
		t.Fatalf("GobSerializer.Deserialize expected non nil error, it return error: %v", err)
	}

	if !reflect.DeepEqual(sess, sess2) {
		t.Fatal("GobSerializer Serialize(Deserialize(sess)) != sess")
	}
}

func init() {
	gob.Register(testKey{})
}
