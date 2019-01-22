package sersan

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

var tntError = errors.New("Tnt Storage method used")

type tntStorage struct{}

func (s *tntStorage) Get(id string) (*Session, error)        { return nil, tntError }
func (s *tntStorage) Destroy(id string) error                { return tntError }
func (s *tntStorage) DestroyAllOfAuthId(authId string) error { return tntError }
func (s *tntStorage) Insert(sess *Session) error             { return tntError }
func (s *tntStorage) Replace(sess *Session) error            { return tntError }

func TestLoadSession(t *testing.T) {
	tests := []struct {
		storage Storage
		cookie  string
	}{
		{&tntStorage{}, ""},
		{NewStorageRecorder(), "123456789-123456789-123456789-12"},
	}

	for i, test := range tests {
		ss := NewServerSessionState(test.storage)
		data, token, err := ss.Load(test.cookie)

		if err != nil {
			t.Errorf("%d: Load session failed with error: %v", i, err)
		}

		if token.sess != nil {
			t.Errorf("%d: Expected SaveSessionToken to be nill", i)
		}

		if len(data) != 0 {
			t.Errorf("%d: Expected empty session data, got map with len %d", i, len(data))
		}
	}
}

func TestLoadSessionExists(t *testing.T) {
	sess := NewSession("123456789-123456789-123456789-12", "auth-id", time.Now().UTC())
	sess.Values["foo"] = "bar"
	storage := PrepareStorageRecorder([]*Session{
		sess,
	})
	ss := NewServerSessionState(storage)
	data, token, err := ss.Load("123456789-123456789-123456789-12")
	if err != nil {
		t.Errorf("Load session failed with error: %v", err)
	}

	if v, ok := data["foo"]; !ok || v != "bar" {
		t.Errorf("Expected session data contains 'foo' key with value 'bar'. Got: %v", data)
	}

	if v, ok := data[ss.AuthKey]; !ok || v != "auth-id" {
		t.Errorf("Expected session data contains '%s' key with value 'auth-id'. Got: %v", ss.AuthKey, data)
	}

	if !reflect.DeepEqual(sess, token.sess) {
		t.Error("Expected token session to deep equal session returned by storage")
	}
}

func TestNextExpires(t *testing.T) {
	var stnt = func(i, a int) *ServerSessionState {
		ss := NewServerSessionState(&tntStorage{})
		ss.IdleTimeout = i
		ss.AbsoluteTimeout = a

		return ss
	}

	var session = func(a, c time.Time) *Session {
		sess := NewSession("irr", "irr", a)
		sess.CreatedAt = c

		return sess
	}

	fakenow, _ := time.Parse("2006-01-02 15:04:05 MST", "2015-05-27 17:55:41 UTC")

	var zero time.Time

	tests := []struct {
		iddle, absolute                int
		accessedAt, createdAt, expires time.Time
	}{
		{0, 0, zero, zero, zero},
		{1, 0, fakenow, zero, fakenow.Add(time.Second)},
		{0, 1, zero, fakenow, fakenow.Add(time.Second)},
		{3, 7, fakenow, fakenow, fakenow.Add(time.Second * 3)},
		{3, 7, fakenow.Add(time.Second * 4), fakenow, fakenow.Add(time.Second * 7)},
		{3, 7, fakenow.Add(time.Second * 5), fakenow, fakenow.Add(time.Second * 7)},
	}

	var (
		expires time.Time
		ss      *ServerSessionState
	)
	for i, test := range tests {
		ss = stnt(test.iddle, test.absolute)
		expires = ss.NextExpires(session(test.accessedAt, test.createdAt))
		if !expires.Equal(test.expires) {
			t.Errorf("%d: expected %v to be equal %v", i, expires, test.expires)
		}
	}
}

func TestSaveSessionNothing(t *testing.T) {
	storage := NewStorageRecorder()
	ss := NewServerSessionState(storage)
	token := &SaveSessionToken{now: time.Now().UTC(), sess: nil}
	sess, err := ss.Save(token, make(map[interface{}]interface{}))

	if sess != nil {
		t.Error("Expected returned session to be nil")
	}
	if err != nil {
		t.Errorf("Expected non nil err, returned %v", err)
	}

	operations := storage.GetOperations()
	if !reflect.DeepEqual(operations, []*RecorderOperation{}) {
		t.Errorf("expected storage operation to be empty, returned %d instead", len(operations))
	}
}

func TestSaveSessionInitialize(t *testing.T) {
	storage := NewStorageRecorder()
	ss := NewServerSessionState(storage)
	token := &SaveSessionToken{now: time.Now().UTC(), sess: nil}
	data := map[interface{}]interface{}{
		"a": "b",
	}
	sess, err := ss.Save(token, data)
	if sess == nil {
		t.Error("Expected returned session to be non nil")
	}
	if err != nil {
		t.Errorf("Expected non nil err, returned %v", err)
	}

	expectedOp := []*RecorderOperation{
		&RecorderOperation{
			Tag:     "Insert",
			Session: sess,
		},
	}
	if !reflect.DeepEqual(sess.Values, data) {
		t.Error("expected sess.Values to be equals written data")
	}

	if !reflect.DeepEqual(storage.GetOperations(), expectedOp) {
		t.Error("Invalid storage operation")
	}
}

// We already test the other functions that ServerSessionState.Save calls.
// A single unit test just to be sure everything is connected should be enough.
func TestComplexSaveSession(t *testing.T) {
	var op   []*RecorderOperation

	fakenow, _ := time.Parse("2006-01-02 15:04:05 MST", "2015-05-27 17:55:41 UTC")
	emptyMap := make(map[interface{}]interface{})

	storage := NewStorageRecorder()
	ss := NewServerSessionState(storage)
	if sess, err := ss.Save(&SaveSessionToken{now: fakenow, sess: nil}, emptyMap); err != nil || sess != nil {
		t.Fatal("expected save return nill sess and non nil error")
	}
	if op = storage.GetOperations(); !reflect.DeepEqual(op, []*RecorderOperation{}) {
		t.Fatalf("expected empty operation in storage. return %d", len(op))
	}

	m1 := make(map[interface{}]interface{})
	m1["foo"] = "bar"
	sess, err := ss.Save(&SaveSessionToken{now: fakenow, sess: nil}, m1)
	if sess == nil || err != nil {
		t.Fatalf("expected save return no nil session and non nil error")
	}
	if sess.AuthID != "" {
		t.Errorf("expected session.AuthID to be empty, return %s instead", sess.AuthID)
	}
	if !reflect.DeepEqual(sess.Values, m1) {
		t.Error("sess.Values returned is not equals with data passed to Save")
	}
	if op = storage.GetOperations(); !reflect.DeepEqual(op, []*RecorderOperation{
		&RecorderOperation{Tag: "Insert", Session: sess},
	}) {
		t.Error("expected single operation Insert in storage")
	}

	m2 := copyMap(m1)
	m2[ss.AuthKey] = "john"
	sess2, err := ss.Save(&SaveSessionToken{now: fakenow, sess: sess}, m2)
	if sess2 == nil || err != nil {
		t.Fatalf("expected save return no nil session and non nil error")
	}
	if sess2.AuthID != "john" {
		t.Fatalf("expected session auth ID == 'john', actual %s", sess2.AuthID)
	}
	if !reflect.DeepEqual(sess2.Values, m1) {
		t.Fatal("only setting authID didn't update session value")
	}
	if sess2.ID == sess.ID {
		t.Fatal("expected session ID to be different when updating AuthID")
	}

	if op = storage.GetOperations(); !reflect.DeepEqual(op, []*RecorderOperation{
		&RecorderOperation{Tag: "Destroy", ID: sess.ID},
		&RecorderOperation{Tag: "Insert", Session: sess2},
	}) {
		t.Fatal("expected operation Destory, Insert")
	}

	// force invalidate
	m3 := copyMap(m1)
	m3[ss.AuthKey] = "john"
	m3[ForceInvalidateKey] = AllSessionIDsOfLoggedUser
	sess3, err := ss.Save(&SaveSessionToken{now: fakenow, sess: sess2}, m3)
	if sess3 == nil || err != nil {
		t.Fatalf("expected save return no nil session and non nil error")
	}
	var expectedSess = new(Session)
	*expectedSess = *sess2
	expectedSess.ID = sess3.ID
	if !reflect.DeepEqual(sess3, expectedSess) {
		t.Fatal("force invalidate should return similar session except ID.")
	}
	if op = storage.GetOperations(); !reflect.DeepEqual(op, []*RecorderOperation{
		&RecorderOperation{Tag: "Destroy", ID: sess2.ID},
		&RecorderOperation{Tag: "DestroyAllOfAuthId", AuthID: "john"},
		&RecorderOperation{Tag: "Insert", Session: sess3},
	}) {
		t.Fatal("expected operations Destory, Insert")
	}

	m4 := copyMap(m1)
	m4[ss.AuthKey] = "john"
	m4["x"] = "y"
	sess4, err := ss.Save(&SaveSessionToken{now: fakenow, sess: sess3}, m4)
	if sess4 == nil || err != nil {
		t.Fatal("expected save return no nil session and non nil error")
	}
	if op = storage.GetOperations(); !reflect.DeepEqual(op, []*RecorderOperation{
		&RecorderOperation{Tag: "Replace", Session: sess4},
	}) {
		t.Fatal("expected a single operation Replace in storage")
	}
}

func copyMap(m map[interface{}]interface{}) map[interface{}]interface{} {
	m1 := make(map[interface{}]interface{})
	for k, v := range m {
		m1[k] = v
	}
	return m1
}