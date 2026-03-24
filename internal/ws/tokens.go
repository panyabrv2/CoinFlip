package ws

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	ErrTokenEmpty    = errors.New("token empty")
	ErrTokenNotFound = errors.New("token not found")
	ErrTokenLocked   = errors.New("token locked")
)

type TokenStore struct {
	rdb *redis.Client

	lockScript   *redis.Script
	unlockScript *redis.Script
	touchScript  *redis.Script

	opTimeout time.Duration
}

func NewTokenStore(rdb *redis.Client) *TokenStore {
	lockLua := `
local exists = redis.call("EXISTS", KEYS[1])
if exists == 0 then
  return {0, 0}
end

local uid = redis.call("HGET", KEYS[1], "user_id")
if not uid then
  return {0, 0}
end

local locked = redis.call("HGET", KEYS[1], "locked")
if locked and tonumber(locked) ~= 0 then
  return {tonumber(uid), 0}
end

redis.call("HSET", KEYS[1], "locked", 1)
redis.call("HSET", KEYS[1], "session_id", ARGV[1])
redis.call("HSET", KEYS[1], "last_seen", ARGV[2])
return {tonumber(uid), 1}
`

	unlockLua := `
local exists = redis.call("EXISTS", KEYS[1])
if exists == 0 then
  return 0
end

local sid = redis.call("HGET", KEYS[1], "session_id")
if not sid then
  return 0
end

if sid ~= ARGV[1] then
  return 0
end

redis.call("HSET", KEYS[1], "locked", 0)
redis.call("HDEL", KEYS[1], "session_id")
redis.call("HDEL", KEYS[1], "last_seen")
return 1
`

	touchLua := `
local exists = redis.call("EXISTS", KEYS[1])
if exists == 0 then
  return 0
end

local locked = redis.call("HGET", KEYS[1], "locked")
if not locked or tonumber(locked) == 0 then
  return 0
end

local sid = redis.call("HGET", KEYS[1], "session_id")
if not sid then
  return 0
end
if sid ~= ARGV[1] then
  return 0
end

redis.call("HSET", KEYS[1], "last_seen", ARGV[2])
return 1
`

	return &TokenStore{
		rdb:          rdb,
		lockScript:   redis.NewScript(lockLua),
		unlockScript: redis.NewScript(unlockLua),
		touchScript:  redis.NewScript(touchLua),
		opTimeout:    2 * time.Second,
	}
}

func redisTokenKey(token string) string { return "auth_token:" + token }

func newSessionID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (s *TokenStore) LockWithSession(ctx context.Context, token string) (userID int64, sessionID string, ok bool, err error) {
	if s == nil || s.rdb == nil {
		return 0, "", false, fmt.Errorf("token store misconfigured")
	}
	token = stringsTrim(token)
	if token == "" {
		return 0, "", false, ErrTokenEmpty
	}

	sid, err := newSessionID()
	if err != nil {
		return 0, "", false, err
	}

	ctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	now := time.Now().Unix()
	key := redisTokenKey(token)

	res, err := s.lockScript.Run(ctx, s.rdb, []string{key}, sid, now).Result()
	if err != nil {
		return 0, "", false, err
	}

	arr, okArr := res.([]interface{})
	if !okArr || len(arr) != 2 {
		return 0, "", false, fmt.Errorf("unexpected lua result: %T %#v", res, res)
	}

	uid, err := toInt64(arr[0])
	if err != nil || uid <= 0 {
		return 0, "", false, ErrTokenNotFound
	}
	okInt, err := toInt64(arr[1])
	if err != nil {
		return uid, "", false, err
	}
	if okInt == 1 {
		return uid, sid, true, nil
	}
	return uid, "", false, ErrTokenLocked
}

func (s *TokenStore) UnlockWithSession(ctx context.Context, token, sessionID string) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	token = stringsTrim(token)
	sessionID = stringsTrim(sessionID)
	if token == "" || sessionID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	key := redisTokenKey(token)
	_, err := s.unlockScript.Run(ctx, s.rdb, []string{key}, sessionID).Result()
	return err
}

func (s *TokenStore) Touch(ctx context.Context, token, sessionID string) error {
	if s == nil || s.rdb == nil {
		return nil
	}
	token = stringsTrim(token)
	sessionID = stringsTrim(sessionID)
	if token == "" || sessionID == "" {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, s.opTimeout)
	defer cancel()

	now := time.Now().Unix()
	key := redisTokenKey(token)
	_, err := s.touchScript.Run(ctx, s.rdb, []string{key}, sessionID, now).Result()
	return err
}

func (s *TokenStore) CleanupStale(ctx context.Context, staleSeconds int) (int, error) {
	if s == nil || s.rdb == nil {
		return 0, nil
	}
	if staleSeconds <= 0 {
		return 0, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	now := time.Now().Unix()
	var cursor uint64
	unlocked := 0

	for {
		keys, next, err := s.rdb.Scan(ctx, cursor, "auth_token:*", 200).Result()
		if err != nil {
			return unlocked, err
		}

		for _, key := range keys {
			vals, err := s.rdb.HMGet(ctx, key, "locked", "last_seen").Result()
			if err != nil {
				continue
			}

			lockedStr := fmt.Sprint(vals[0])
			if lockedStr == "" || lockedStr == "<nil>" {
				continue
			}
			lockedInt, _ := strconv.ParseInt(stringsTrim(lockedStr), 10, 64)
			if lockedInt == 0 {
				continue
			}

			lastSeenStr := fmt.Sprint(vals[1])
			if lastSeenStr == "" || lastSeenStr == "<nil>" {
				_, _ = s.rdb.HSet(ctx, key, "locked", 0).Result()
				_, _ = s.rdb.HDel(ctx, key, "session_id", "last_seen").Result()
				unlocked++
				continue
			}

			lastSeen, err := strconv.ParseInt(stringsTrim(lastSeenStr), 10, 64)
			if err != nil {
				continue
			}

			if now-lastSeen >= int64(staleSeconds) {
				_, _ = s.rdb.HSet(ctx, key, "locked", 0).Result()
				_, _ = s.rdb.HDel(ctx, key, "session_id", "last_seen").Result()
				unlocked++
			}
		}

		cursor = next
		if cursor == 0 {
			break
		}
	}

	return unlocked, nil
}

func toInt64(v interface{}) (int64, error) {
	switch t := v.(type) {
	case int64:
		return t, nil
	case int:
		return int64(t), nil
	case string:
		return strconv.ParseInt(stringsTrim(t), 10, 64)
	case []byte:
		return strconv.ParseInt(stringsTrim(string(t)), 10, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to int64", v)
	}
}

func stringsTrim(s string) string {
	i := 0
	j := len(s)
	isTrim := func(c byte) bool {
		return c == ' ' || c == '\n' || c == '\r' || c == '\t' || c == '~'
	}
	for i < j && isTrim(s[i]) {
		i++
	}
	for j > i && isTrim(s[j-1]) {
		j--
	}
	return s[i:j]
}
