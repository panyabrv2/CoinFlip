package game

import (
	"strconv"
	"sync"
	"time"
)

type ItemRef struct {
	Type    string  `json:"type"`
	ItemID  string  `json:"item_id"`
	CostTon float64 `json:"cost_ton"`
}

type BetItemSnapshot struct {
	Type     string  `json:"type"`
	ItemID   string  `json:"item_id"`
	Name     string  `json:"name"`
	PhotoURL *string `json:"photo_url"`
	CostTon  float64 `json:"cost_ton"`
}

type BetSnapshot struct {
	BetID     int             `json:"bet_id"`
	BetType   string          `json:"bet_type"`
	CreatedAt string          `json:"created_at"`
	UserID    int64           `json:"user_id"`
	Side      string          `json:"side"`
	BetItem   BetItemSnapshot `json:"bet_item"`
}

type UserBetsSnapshot struct {
	PhotoURL string        `json:"photo_url"`
	Bets     []BetSnapshot `json:"bets"`
}

type BetStore struct {
	mu sync.RWMutex

	bets map[int]map[int64]*UserBetsSnapshot

	nextBetID map[int]int
}

func NewBetStore() *BetStore {
	return &BetStore{
		bets:      make(map[int]map[int64]*UserBetsSnapshot),
		nextBetID: make(map[int]int),
	}
}

func (s *BetStore) Add(gameID int, userID int64, side string, items []ItemRef) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.bets[gameID]; !ok {
		s.bets[gameID] = make(map[int64]*UserBetsSnapshot)
	}
	if _, ok := s.bets[gameID][userID]; !ok {
		s.bets[gameID][userID] = &UserBetsSnapshot{
			PhotoURL: "",
			Bets:     []BetSnapshot{},
		}
	}

	accepted := 0

	createdAt := time.Now().UTC().Format("2006-01-02 15:04:05.000000-07")

	for _, it := range items {
		s.nextBetID[gameID]++
		betID := s.nextBetID[gameID]

		b := BetSnapshot{
			BetID:     betID,
			BetType:   it.Type,
			CreatedAt: createdAt,
			UserID:    userID,
			Side:      side,
			BetItem: BetItemSnapshot{
				Type:     it.Type,
				ItemID:   it.ItemID,
				Name:     it.ItemID,
				PhotoURL: nil,
				CostTon:  it.CostTon,
			},
		}

		s.bets[gameID][userID].Bets = append(s.bets[gameID][userID].Bets, b)
		accepted++
	}

	return accepted
}

func (s *BetStore) UniqueUsers(gameID int) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.bets[gameID] == nil {
		return 0
	}
	return len(s.bets[gameID])
}

func (s *BetStore) Snapshot(gameID int) map[string]UserBetsSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make(map[string]UserBetsSnapshot)

	m := s.bets[gameID]
	if m == nil {
		return out
	}

	for uid, ub := range m {
		if ub == nil {
			continue
		}
		out[strconv.FormatInt(uid, 10)] = UserBetsSnapshot{
			PhotoURL: ub.PhotoURL,
			Bets:     append([]BetSnapshot(nil), ub.Bets...),
		}
	}

	return out
}

func (s *BetStore) Reset(gameID int) {
	s.mu.Lock()
	delete(s.bets, gameID)
	delete(s.nextBetID, gameID)
	s.mu.Unlock()
}
