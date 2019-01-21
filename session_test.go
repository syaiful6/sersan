package sersan

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

// NewRecorder returns an initialized ResponseRecorder.
func NewRecorder() *httptest.ResponseRecorder {
	return &httptest.ResponseRecorder{
		HeaderMap: make(http.Header),
		Body:      new(bytes.Buffer),
	}
}

var tntError = errors.New("Tnt Storage method used")

type tntStorage struct{}

func (s *tntStorage) Get(id string) (*Session, error)        { return nil, tntError }
func (s *tntStorage) Destroy(id string) error                { return tntError }
func (s *tntStorage) DestroyAllOfAuthId(authId string) error { return tntError }
func (s *tntStorage) Insert(sess *Session) error             { return tntError }
func (s *tntStorage) Replace(sess *Session) error            { return tntError }

type mockOperation struct {
	tag, id, authId string
	session         *Session
}

type mockStorage struct {
	sessions map[string]*Session
	op       []*mockOperation
}

func (s *mockStorage) Get(id string) (*Session, error) {
	s.op = append(s.op, &mockOperation{tag: "Get", id: id})
	if v, ok := s.sessions[id]; ok {
		return v, nil
	}

	return nil, nil
}

func (s *mockStorage) Destroy(id string) error {
	s.op = append(s.op, &mockOperation{tag: "Destroy", id: id})
	if _, ok := s.sessions[id]; ok {
		delete(s.sessions, id)
		return nil
	}

	return nil
}

func (s *mockStorage) DestroyAllOfAuthId(authId string) error {
	nmap := make(map[string]*Session)
	for k, ses := range s.sessions {
		if ses.AuthID != authId {
			nmap[k] = ses
		}
	}
	s.sessions = nmap
	s.op = append(s.op, &mockOperation{tag: "DestroyAllOfAuthId", authId: authId})

	return nil
}

func (s *mockStorage) Insert(sess *Session) error {
	s.op = append(s.op, &mockOperation{tag: "Insert", session: sess})
	if old, ok := s.sessions[sess.ID]; ok {
		return &SessionAlreadyExists{OldSession: old, NewSession: sess}
	}

	s.sessions[sess.ID] = sess
	return nil
}

func (s *mockStorage) Replace(sess *Session) error {
	s.op = append(s.op, &mockOperation{tag: "Replace", session: sess})
	if _, ok := s.sessions[sess.ID]; ok {
		s.sessions[sess.ID] = sess
		return nil
	}

	return &SessionDoesNotExist{Session: sess}
}

func prepareMockStorage(sessions []*Session) *mockStorage {
	sess := make(map[string]*Session)
	for _, s := range sessions {
		sess[s.ID] = s
	}

	return &mockStorage{
		sessions: sess,
		op:       []*mockOperation{},
	}
}

func TestLoadSession(t *testing.T) {
	tests := []struct {
		storage Storage
		cookie  string
	}{
		{&tntStorage{}, ""},
		{prepareMockStorage([]*Session{}), "123456789-123456789-123456789-12"},
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
	storage := prepareMockStorage([]*Session{
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
