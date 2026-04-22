package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"go.uber.org/zap"
)

type Session struct {
	Email     string
	ExpiresAt time.Time
}

type PersistedSession struct {
	Token     string
	Email     string
	ExpiresAt time.Time
}

type SessionPersister interface {
	SaveSession(ctx context.Context, token, email string, expiresAt time.Time) error
	LoadSessions(ctx context.Context) ([]PersistedSession, error)
}

type TokenStore struct {
	mu        sync.RWMutex
	sessions  map[string]Session
	persister SessionPersister
	log       *zap.Logger
}

func NewTokenStore(persister SessionPersister, log *zap.Logger) *TokenStore {
	s := &TokenStore{
		sessions:  make(map[string]Session),
		persister: persister,
		log:       log,
	}

	if persister != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		rows, err := persister.LoadSessions(ctx)
		if err != nil {
			log.Error("failed to load sessions from db", zap.Error(err))
		} else {
			now := time.Now()
			for _, r := range rows {
				if r.ExpiresAt.After(now) {
					s.sessions[r.Token] = Session{Email: r.Email, ExpiresAt: r.ExpiresAt}
				}
			}
			log.Info("loaded sessions from db", zap.Int("count", len(s.sessions)))
		}
	}

	return s
}

func (s *TokenStore) Create(email string, ttl time.Duration) (string, Session, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", Session{}, err
	}
	token := hex.EncodeToString(b)
	sess := Session{Email: email, ExpiresAt: time.Now().Add(ttl)}

	s.mu.Lock()
	s.sessions[token] = sess
	s.mu.Unlock()

	if s.persister != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := s.persister.SaveSession(ctx, token, email, sess.ExpiresAt); err != nil {
			s.log.Error("failed to persist session", zap.String("email", email), zap.Error(err))
		}
	}

	return token, sess, nil
}

func (s *TokenStore) Validate(token string) (Session, bool) {
	s.mu.RLock()
	sess, ok := s.sessions[token]
	s.mu.RUnlock()

	if !ok || time.Now().After(sess.ExpiresAt) {
		return Session{}, false
	}
	return sess, true
}

func (s *TokenStore) Cleanup() {
	s.mu.Lock()
	now := time.Now()
	for k, v := range s.sessions {
		if now.After(v.ExpiresAt) {
			delete(s.sessions, k)
		}
	}
	s.mu.Unlock()
}
