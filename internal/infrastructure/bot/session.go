// Package bot implements the portal chat bot — rule-based FAQ → Ollama AI → live handoff.
package bot

import (
	"sync"
	"time"
)

// States for a bot session.
const (
	StateBot     = "bot"     // bot is handling the conversation
	StateHandoff = "handoff" // waiting for a staff member to join
	StateLive    = "live"    // connected to a staff member
	StateClosed  = "closed"  // conversation ended
)

// BotMessage is a single turn in the conversation.
type BotMessage struct {
	Role    string // "user" or "assistant"
	Content string
	At      time.Time
}

// BotSession tracks an ongoing portal customer chat.
type BotSession struct {
	ID         string
	CustomerID string
	Messages   []BotMessage
	State      string
	ThreadID   string // set when live handoff creates a chat thread
	LastActive time.Time
}

// Sessions is an in-memory store for portal bot sessions, keyed by session ID.
type Sessions struct {
	mu       sync.Mutex
	sessions map[string]*BotSession
	ttl      time.Duration
}

// NewSessions creates a session store with the given TTL (0 → 30 min).
func NewSessions(ttl time.Duration) *Sessions {
	if ttl == 0 {
		ttl = 30 * time.Minute
	}
	s := &Sessions{
		sessions: make(map[string]*BotSession),
		ttl:      ttl,
	}
	return s
}

// GetOrCreate returns an existing session or creates a new one.
func (s *Sessions) GetOrCreate(sessionID, customerID string) *BotSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		sess = &BotSession{
			ID:         sessionID,
			CustomerID: customerID,
			State:      StateBot,
			LastActive: time.Now(),
		}
		s.sessions[sessionID] = sess
	}
	sess.LastActive = time.Now()
	return sess
}

// Get returns an existing session or nil.
func (s *Sessions) Get(sessionID string) *BotSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess := s.sessions[sessionID]
	if sess != nil {
		sess.LastActive = time.Now()
	}
	return sess
}

// MessagesSnapshot returns a copy of a session's message history under the lock.
// Returns nil if the session does not exist.
func (s *Sessions) MessagesSnapshot(sessionID string) []BotMessage {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return nil
	}
	out := make([]BotMessage, len(sess.Messages))
	copy(out, sess.Messages)
	return out
}

// UpdateState atomically updates State and optionally ThreadID for a session.
// No-op if the session does not exist.
func (s *Sessions) UpdateState(sessionID, state, threadID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	if state != "" {
		sess.State = state
	}
	if threadID != "" {
		sess.ThreadID = threadID
	}
}

// AppendMessages atomically appends messages to a session's history.
// No-op if the session does not exist.
func (s *Sessions) AppendMessages(sessionID string, msgs ...BotMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sess, ok := s.sessions[sessionID]
	if !ok {
		return
	}
	sess.Messages = append(sess.Messages, msgs...)
}

// Delete removes a session.
func (s *Sessions) Delete(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, sessionID)
}

// Expired returns sessions that have exceeded the TTL.
func (s *Sessions) Expired() []*BotSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []*BotSession
	cutoff := time.Now().Add(-s.ttl)
	for _, sess := range s.sessions {
		if sess.LastActive.Before(cutoff) && sess.State != StateClosed {
			out = append(out, sess)
		}
	}
	return out
}
