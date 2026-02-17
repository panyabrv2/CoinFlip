package game

import "sync"

type ItemRef struct {
	Type   string `json:"type"`
	ItemID string `json:"item_id"`
}

type UserBet struct {
	UserID int64     `json:"user_id"`
	Side   string    `json:"side"`
	Items  []ItemRef `json:"items"`
}

type BetStore struct {
	mu   sync.RWMutex
	bets map[int]map[int64]UserBet
}

func NewBetStore() *BetStore {
	return &BetStore{
		bets: make(map[int]map[int64]UserBet),
	}
}

func (s *BetStore) Add(gameID int, userID int64, side string, items []ItemRef) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.bets[gameID]; !ok {
		s.bets[gameID] = make(map[int64]UserBet)
	}
	s.bets[gameID][userID] = UserBet{
		UserID: userID,
		Side:   side,
		Items:  items,
	}
	return len(items)
}

func (s *BetStore) UniqueUsers(gameID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return len(s.bets[gameID])
}

func (s *BetStore) Snapshot(gameID int) any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]UserBet, 0, len(s.bets[gameID]))
	for _, b := range s.bets[gameID] {
		out = append(out, b)
	}
	return out
}

func (s *BetStore) Reset(gameID int) {
	s.mu.Lock()
	delete(s.bets, gameID)
	s.mu.Unlock()
}
