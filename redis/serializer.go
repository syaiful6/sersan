package redis

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"fmt"

	"github.com/syaiful6/sersan"
)

type SessionSerializer interface {
	Serialize(s *sersan.Session) ([]byte, error)
	Deserialize(b []byte, s *sersan.Session) error
}

type JSONSerializer struct{}

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
	return json.Marshal(m)
}

func (js JSONSerializer) Deserialize(b []byte, ss *sersan.Session) error {
	m := make(map[string]interface{})
	err := json.Unmarshal(b, &m)
	if err != nil {
		fmt.Printf("sersan.redis.JSONSerializer.deserialize() Error: %v", err)
		return err
	}
	if ss.Values == nil {
		ss.Values = make(map[interface{}]interface{})
	}
	for k, v := range m {
		ss.Values[k] = v
	}
	return nil
}

type GobSerializer struct{}

func (g GobSerializer) Serialize(ss *sersan.Session) ([]byte, error) {
	buf := new(bytes.Buffer)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(ss.Values)
	if err == nil {
		return buf.Bytes(), nil
	}
	return nil, err
}

func (g GobSerializer) Deserialize(b []byte, ss *sersan.Session) error {
	dec := gob.NewDecoder(bytes.NewBuffer(b))
	return dec.Decode(&ss.Values)
}
