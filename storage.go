package sersan

// A storage backend, for server-side sessions.
type Storage interface {
	// Get the session for the given session ID. Returns nil if it not exists
	// rather than returning error
	Get(id string) (*Session, error)
	// Delete the session with given session ID. Does not do anything if the session
	// is not found.
	Destroy(id string) error
	// Delete all sessions of the given auth ID. Does not do anything if there
	// are no sessions of the given auth ID.
	DestroyAllOfAuthId(authId string) error
	// Insert a new session. return 'SessionAlreadyExists' error there already
	// exists a session with the same session ID. We only call this method after
	// generating a fresh session ID
	Insert(sess *Session) error
	// Replace the contents of a session. Return 'SessionDoesNotExist' if
	// there is no session with the given  session ID
	Replace(sess *Session) error
}

// Operation item in StorageRecorder, represent mock operation that was executed.
type RecorderOperation struct {
	Tag, ID, AuthID string
	Session         *Session
}

// Storage recorder implements sersan's storage interface that record all operations
// performed. This is intended for testing purpose.
type StorageRecorder struct {
	sessions map[string]*Session
	operations []*RecorderOperation
}

// NewRecorder returns an empty StorageRecorder
func NewStorageRecorder() *StorageRecorder {
	return &StorageRecorder{
		sessions: make(map[string]*Session),
		operations:  []*RecorderOperation{},
	}
}

// Create StorageRecorder with predefined `Session`s.
func PrepareStorageRecorder(sessions []*Session) *StorageRecorder {
	sess := make(map[string]*Session)
	for _, s := range sessions {
		sess[s.ID] = s
	}

	return &StorageRecorder{
		sessions: sess,
		operations: []*RecorderOperation{},
	}
}

func (s *StorageRecorder) Get(id string) (*Session, error) {
	s.operations = append(s.operations, &RecorderOperation{Tag: "Get", ID: id,})
	if v, ok := s.sessions[id]; ok {
		return v, nil
	}

	return nil, nil
}

func (s *StorageRecorder) Destroy(id string) error {
	if _, ok := s.sessions[id]; ok {
		delete(s.sessions, id)
	}
	s.operations = append(s.operations, &RecorderOperation{Tag: "Destroy", ID: id,})

	return nil
}

func (s *StorageRecorder) DestroyAllOfAuthId(authId string) error {
	nmap := make(map[string]*Session)
	for k, sess := range s.sessions {
		if sess.AuthID != authId {
			nmap[k] = sess
		}
	}
	s.sessions = nmap
	s.operations = append(s.operations, &RecorderOperation{Tag: "DestroyAllOfAuthId", AuthID: authId,})

	return nil
}

func (s *StorageRecorder) Insert(sess *Session) error {
	s.operations = append(s.operations, &RecorderOperation{Tag: "Insert", Session: sess,})
	if old, ok := s.sessions[sess.ID]; ok {
		return &SessionAlreadyExists{OldSession: old, NewSession: sess,}
	}

	s.sessions[sess.ID] = sess
	return nil
}

func (s *StorageRecorder) Replace(sess *Session) error {
	s.operations = append(s.operations, &RecorderOperation{Tag: "Replace", Session: sess,})
	if _, ok := s.sessions[sess.ID]; ok {
		s.sessions[sess.ID] = sess
		return nil
	}

	return &SessionDoesNotExist{Session: sess,}
}

// Get list of Operations performed in StorageRecorder, remove it from the storage
// before returned.
func (s *StorageRecorder) GetOperations() []*RecorderOperation {
	operations := s.operations
	s.operations = []*RecorderOperation{}

	return operations
}