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

type betsSnapshot struct {
	TotalBets  int        `json:"total_bets"`
	TotalValue int        `json:"total_value"`
	Heads      sideBucket `json:"heads"`
	Tails      sideBucket `json:"tails"`
}

type sideBucket struct {
	TotalBets  int        `json:"total_bets"`
	TotalValue int        `json:"total_value"`
	Users      []userSlot `json:"users"`
}

type userSlot struct {
	UserID     int64     `json:"user_id"`
	TotalValue int       `json:"total_value"`
	Items      []ItemRef `json:"items"`
}

func (s *BetStore) Snapshot(gameID int) any {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out betsSnapshot

	for _, b := range s.bets[gameID] {
		slot := userSlot{
			UserID:     b.UserID,
			TotalValue: 0,
			Items:      b.Items,
		}

		out.TotalBets++

		if b.Side == "heads" {
			out.Heads.TotalBets++
			out.Heads.Users = append(out.Heads.Users, slot)
		} else if b.Side == "tails" {
			out.Tails.TotalBets++
			out.Tails.Users = append(out.Tails.Users, slot)
		}
	}

	out.TotalValue = 0
	out.Heads.TotalValue = 0
	out.Tails.TotalValue = 0

	if out.Heads.Users == nil {
		out.Heads.Users = []userSlot{}
	}
	if out.Tails.Users == nil {
		out.Tails.Users = []userSlot{}
	}

	return out
}

func (s *BetStore) Reset(gameID int) {
	s.mu.Lock()
	delete(s.bets, gameID)
	s.mu.Unlock()
}
