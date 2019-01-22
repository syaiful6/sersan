package redis

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/syaiful6/sersan"
)

type SessionSerializer interface {
	Serialize(s *sersan.Session) ([]byte, error)
	Deserialize(b []byte, s *sersan.Session) error
}

type JSONSerializer struct{}

type JSONSession struct {
	ID, AuthID            string
	CreatedAt, AccessedAt time.Time
	Values                map[string]interface{}
}

func (js JSONSerializer) Serialize(s *sersan.Session) ([]byte, error) {
	m := make(map[string]interface{}, len(s.Values))
	for k, v := range s.Values {
		ks, ok := k.(string)
		if !ok {
			err := fmt.Errorf("Non-string key value, cannot serialize session to JSON: %v", k)
			return nil, err
		}
		m[ks] = v
	}

	jses := JSONSession{
		ID:         s.ID,
		AuthID:     s.AuthID,
		CreatedAt:  s.CreatedAt,
		AccessedAt: s.AccessedAt,
		Values:     m,
	}
	return json.Marshal(jses)
}

func (js JSONSerializer) Deserialize(b []byte, s *sersan.Session) error {
	jses := JSONSession{}
	err := json.Unmarshal(b, &jses)
	if err != nil {
		return err
	}
	// copy
	s.ID = jses.ID
	s.AuthID = jses.AuthID
	s.CreatedAt = jses.CreatedAt
	s.AccessedAt = jses.AccessedAt
	if s.Values == nil {
		s.Values = make(map[interface{}]interface{})
	}
	for k, v := range jses.Values {
		s.Values[k] = v
	}

	return nil
}

type GobSerializer struct{}

func (g GobSerializer) Serialize(s *sersan.Session) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(s)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

func (g GobSerializer) Deserialize(b []byte, s *sersan.Session) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	return dec.Decode(&s)
}
