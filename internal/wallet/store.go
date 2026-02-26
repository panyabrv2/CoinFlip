package wallet

import (
	"fmt"
	"sync"
)

type Balance struct {
	UserID    int64   `json:"user_id"`
	Available float64 `json:"available"`
	Reserved  float64 `json:"reserved"`
	Total     float64 `json:"total"`
}
type Store struct {
	mu sync.RWMutex

	startBalance   float64
	houseBalance   float64
	available      map[int64]float64
	reservedByGame map[int]map[int64]float64
}

func NewStore(startBalance float64) *Store {
	return &Store{
		startBalance:   startBalance,
		available:      make(map[int64]float64),
		reservedByGame: make(map[int]map[int64]float64),
		houseBalance:   0,
	}
}
func (s *Store) EnsureUser(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if userID == 0 {
		return
	}
	if _, ok := s.available[userID]; !ok {
		s.available[userID] = s.startBalance
	}
}

func (s *Store) Get(userID int64) Balance {
	s.mu.Lock()
	defer s.mu.Unlock()

	avail := s.available[userID]
	var res float64
	for _, m := range s.reservedByGame {
		res += m[userID]
	}

	return Balance{
		UserID:    userID,
		Available: avail,
		Reserved:  res,
		Total:     avail + res,
	}
}

func (s *Store) HouseBalance() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.houseBalance
}

func (s *Store) TryReserve(gameID int, userID int64, amount float64) error {
	if userID == 0 {
		return fmt.Errorf("user_id=0")
	}
	if gameID <= 0 {
		return fmt.Errorf("invalid game_id")
	}
	if amount <= 0 {
		return fmt.Errorf("invalid reserve amount")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.available[userID]; !ok {
		s.available[userID] = s.startBalance
	}

	if s.available[userID] < amount {
		return fmt.Errorf("insufficient funds: have %.6f need %.6f", s.available[userID], amount)
	}

	s.available[userID] -= amount

	if _, ok := s.reservedByGame[gameID]; !ok {
		s.reservedByGame[gameID] = make(map[int64]float64)
	}
	s.reservedByGame[gameID][userID] += amount

	return nil
}

func (s *Store) Unreserve(gameID int, userID int64, amount float64) {
	if userID == 0 || gameID <= 0 || amount <= 0 {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	m := s.reservedByGame[gameID]
	if m == nil {
		return
	}
	cur := m[userID]
	if cur <= 0 {
		return
	}

	if amount >= cur {
		amount = cur
	}

	m[userID] = cur - amount
	s.available[userID] += amount

	if m[userID] <= 0 {
		delete(m, userID)
	}
	if len(m) == 0 {
		delete(s.reservedByGame, gameID)
	}
}

func (s *Store) SettleGame(gameID int, winners map[int64]float64, houseCut float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.reservedByGame, gameID)

	for uid, payout := range winners {
		if payout <= 0 {
			continue
		}
		if _, ok := s.available[uid]; !ok {
			s.available[uid] = s.startBalance
		}
		s.available[uid] += payout
	}

	if houseCut > 0 {
		s.houseBalance += houseCut
	}
}
