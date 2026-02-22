package ws

import "sync"

type TokenStore struct {
	mu   sync.Mutex
	used map[string]int64
	next int64
}

func NewTokenStore(startUserID int64) *TokenStore {
	if startUserID <= 0 {
		startUserID = 1
	}
	return &TokenStore{
		used: make(map[string]int64),
		next: startUserID,
	}
}

func (s *TokenStore) Consume(token string) (userID int64, ok bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if token == "" {
		return 0, false
	}
	if _, exists := s.used[token]; exists {
		return 0, false
	}

	uid := s.next
	s.next++
	s.used[token] = uid
	return uid, true
}
