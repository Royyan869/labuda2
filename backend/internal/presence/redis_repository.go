package presence

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	pkgredis "github.com/labuda/backend/pkg/redis"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type RedisRepository struct {
	client *pkgredis.Client
	log    *zap.Logger
}

func NewRedisRepository(client *pkgredis.Client, log *zap.Logger) *RedisRepository {
	if log == nil {
		log = zap.NewNop()
	}
	return &RedisRepository{client: client, log: log}
}

func leaseKey(userID uuid.UUID) string { return RedisLeaseKeyPrefix + userID.String() }
func stateKey(userID uuid.UUID) string { return RedisStateKeyPrefix + userID.String() }

const leaseMutationLua = `
local leasesKey = KEYS[1]
local deadlinesKey = KEYS[2]
local stateKey = KEYS[3]
local userID = ARGV[1]
local action = ARGV[2]
local connectionID = ARGV[3]
local expiryMs = tonumber(ARGV[4])
local nowMs = tonumber(ARGV[5])

local function load_state()
  local exists = redis.call('EXISTS', stateKey)
  local online = 0
  local version = 0
  if exists == 1 then
    local onlineRaw = redis.call('HGET', stateKey, 'online')
    local versionRaw = redis.call('HGET', stateKey, 'version')
    if onlineRaw == false or versionRaw == false then
      return nil, nil, 'PRESENCE_MALFORMED_STATE'
    end
    online = tonumber(onlineRaw)
    version = tonumber(versionRaw)
    if online == nil or (online ~= 0 and online ~= 1) or version == nil then
      return nil, nil, 'PRESENCE_MALFORMED_STATE'
    end
  end
  return online, version, nil
end

local function max_deadline()
  local top = redis.call('ZREVRANGE', leasesKey, 0, 0, 'WITHSCORES')
  if top[1] == nil then
    return nil
  end
  return tonumber(top[2])
end

redis.call('ZREMRANGEBYSCORE', leasesKey, '-inf', nowMs)

if action == 'resume' then
  redis.call('ZADD', leasesKey, expiryMs, connectionID)
elseif action == 'leave' then
  redis.call('ZREM', leasesKey, connectionID)
else
  return redis.error_reply('PRESENCE_INVALID_ACTION')
end

local activeCount = redis.call('ZCARD', leasesKey)
local currentOnline, version, stateErr = load_state()
if stateErr ~= nil then
  return redis.error_reply(stateErr)
end

local transitioned = 0
local lastSeenMs = false
local nextDeadlineMs = false

if activeCount > 0 then
  nextDeadlineMs = max_deadline()
  if currentOnline == 0 then
    currentOnline = 1
    version = version + 1
    transitioned = 1
  end
  redis.call('HSET', stateKey, 'online', currentOnline, 'version', version)
  if nextDeadlineMs ~= nil then
    redis.call('ZADD', deadlinesKey, nextDeadlineMs, userID)
  end
else
  redis.call('ZREM', deadlinesKey, userID)
  if currentOnline == 1 then
    currentOnline = 0
    version = version + 1
    transitioned = 1
    lastSeenMs = nowMs
  end
  redis.call('HSET', stateKey, 'online', currentOnline, 'version', version)
end

return { activeCount, currentOnline, transitioned, version, nextDeadlineMs, lastSeenMs }
`

const bumpVersionLua = `
local stateKey = KEYS[1]
local exists = redis.call('EXISTS', stateKey)
local online = 0
local version = 0
if exists == 1 then
  local onlineRaw = redis.call('HGET', stateKey, 'online')
  local versionRaw = redis.call('HGET', stateKey, 'version')
  if onlineRaw == false or versionRaw == false then
    return redis.error_reply('PRESENCE_MALFORMED_STATE')
  end
  online = tonumber(onlineRaw)
  version = tonumber(versionRaw)
  if online == nil or (online ~= 0 and online ~= 1) or version == nil then
    return redis.error_reply('PRESENCE_MALFORMED_STATE')
  end
end
version = version + 1
redis.call('HSET', stateKey, 'online', online, 'version', version)
return { online, version }
`

const claimDueUsersLua = `
local deadlinesKey = KEYS[1]
local nowMs = tonumber(ARGV[1])
local limit = tonumber(ARGV[2])
local rows = redis.call('ZRANGEBYSCORE', deadlinesKey, '-inf', nowMs, 'WITHSCORES', 'LIMIT', 0, limit)
if #rows == 0 then
  return {}
end
for i = 1, #rows, 2 do
  redis.call('ZREM', deadlinesKey, rows[i])
end
return rows
`

const sweepUserLua = `
local leasesKey = KEYS[1]
local deadlinesKey = KEYS[2]
local stateKey = KEYS[3]
local userID = ARGV[1]
local nowMs = tonumber(ARGV[2])
local dueAtMs = tonumber(ARGV[3])

local function load_state()
  local exists = redis.call('EXISTS', stateKey)
  local online = 0
  local version = 0
  if exists == 1 then
    local onlineRaw = redis.call('HGET', stateKey, 'online')
    local versionRaw = redis.call('HGET', stateKey, 'version')
    if onlineRaw == false or versionRaw == false then
      return nil, nil, 'PRESENCE_MALFORMED_STATE'
    end
    online = tonumber(onlineRaw)
    version = tonumber(versionRaw)
    if online == nil or (online ~= 0 and online ~= 1) or version == nil then
      return nil, nil, 'PRESENCE_MALFORMED_STATE'
    end
  end
  return online, version, nil
end

local function max_deadline()
  local top = redis.call('ZREVRANGE', leasesKey, 0, 0, 'WITHSCORES')
  if top[1] == nil then
    return nil
  end
  return tonumber(top[2])
end

redis.call('ZREMRANGEBYSCORE', leasesKey, '-inf', nowMs)
local activeCount = redis.call('ZCARD', leasesKey)
local currentOnline, version, stateErr = load_state()
if stateErr ~= nil then
  return redis.error_reply(stateErr)
end

local transitioned = 0
local lastSeenMs = false
local nextDeadlineMs = false

if activeCount > 0 then
  nextDeadlineMs = max_deadline()
  if currentOnline == 0 then
    currentOnline = 1
    version = version + 1
    transitioned = 1
  end
  redis.call('HSET', stateKey, 'online', currentOnline, 'version', version)
  if nextDeadlineMs ~= nil then
    redis.call('ZADD', deadlinesKey, nextDeadlineMs, userID)
  end
else
  redis.call('ZREM', deadlinesKey, userID)
  if currentOnline == 1 then
    currentOnline = 0
    version = version + 1
    transitioned = 1
    lastSeenMs = dueAtMs
  end
  redis.call('HSET', stateKey, 'online', currentOnline, 'version', version)
end

return { activeCount, currentOnline, transitioned, version, nextDeadlineMs, lastSeenMs }
`

func (r *RedisRepository) ResumeLease(ctx context.Context, userID uuid.UUID, connectionID string, now time.Time) (*LeaseResult, error) {
	return r.mutateLease(ctx, "resume", userID, connectionID, now, now.Add(PresenceGracePeriod))
}

func (r *RedisRepository) LeaveLease(ctx context.Context, userID uuid.UUID, connectionID string, now time.Time) (*LeaseResult, error) {
	return r.mutateLease(ctx, "leave", userID, connectionID, now, time.Time{})
}

func (r *RedisRepository) mutateLease(ctx context.Context, action string, userID uuid.UUID, connectionID string, now time.Time, expiry time.Time) (*LeaseResult, error) {
	res, err := r.client.Eval(ctx, leaseMutationLua, []string{leaseKey(userID), RedisDeadlineKey, stateKey(userID)}, userID.String(), action, connectionID, expiry.UTC().UnixMilli(), now.UTC().UnixMilli()).Result()
	if err != nil {
		return nil, fmt.Errorf("presence %s lease: %w", action, err)
	}
	return parseLeaseResult(userID, res)
}

func (r *RedisRepository) BumpVersion(ctx context.Context, userID uuid.UUID) (*LeaseResult, error) {
	res, err := r.client.Eval(ctx, bumpVersionLua, []string{stateKey(userID)}).Result()
	if err != nil {
		return nil, fmt.Errorf("presence bump version: %w", err)
	}
	return parseVersionResult(userID, res)
}

func (r *RedisRepository) ClaimDueUsers(ctx context.Context, limit int, now time.Time) ([]ClaimedDueUser, error) {
	if limit <= 0 {
		return nil, nil
	}
	res, err := r.client.Eval(ctx, claimDueUsersLua, []string{RedisDeadlineKey}, now.UTC().UnixMilli(), limit).Result()
	if err != nil {
		return nil, fmt.Errorf("presence claim due users: %w", err)
	}
	rows, ok := res.([]interface{})
	if !ok || len(rows) == 0 {
		return nil, nil
	}
	out := make([]ClaimedDueUser, 0, len(rows)/2)
	for i := 0; i+1 < len(rows); i += 2 {
		userIDStr, _ := rows[i].(string)
		score := toInt64(rows[i+1])
		parsedID, err := uuid.Parse(userIDStr)
		if err != nil {
			return nil, fmt.Errorf("presence claim due users: invalid user id %q: %w", userIDStr, err)
		}
		out = append(out, ClaimedDueUser{UserID: parsedID, DueAt: time.UnixMilli(score).UTC()})
	}
	return out, nil
}

func (r *RedisRepository) SweepLease(ctx context.Context, userID uuid.UUID, dueAt, now time.Time) (*LeaseResult, error) {
	res, err := r.client.Eval(ctx, sweepUserLua, []string{leaseKey(userID), RedisDeadlineKey, stateKey(userID)}, userID.String(), now.UTC().UnixMilli(), dueAt.UTC().UnixMilli()).Result()
	if err != nil {
		return nil, fmt.Errorf("presence sweep lease: %w", err)
	}
	return parseLeaseResult(userID, res)
}

func (r *RedisRepository) Publish(ctx context.Context, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("presence publish marshal: %w", err)
	}
	if err := r.client.Publish(ctx, RedisEventsChannel, payload).Err(); err != nil {
		return fmt.Errorf("presence publish: %w", err)
	}
	return nil
}

func (r *RedisRepository) Subscribe(ctx context.Context) (*goredis.PubSub, error) {
	ps := r.client.Subscribe(ctx, RedisEventsChannel)
	if _, err := ps.Receive(ctx); err != nil {
		_ = ps.Close()
		return nil, fmt.Errorf("presence subscribe: %w", err)
	}
	return ps, nil
}

func (r *RedisRepository) GetStatesBatch(ctx context.Context, userIDs []uuid.UUID) (map[uuid.UUID]State, error) {
	result := make(map[uuid.UUID]State, len(userIDs))
	if len(userIDs) == 0 {
		return result, nil
	}

	pipe := r.client.Pipeline()
	cmds := make([]*goredis.MapStringStringCmd, 0, len(userIDs))
	for _, userID := range userIDs {
		cmds = append(cmds, pipe.HGetAll(ctx, stateKey(userID)))
	}
	if _, err := pipe.Exec(ctx); err != nil && !strings.Contains(err.Error(), "EXECABORT") {
		return nil, fmt.Errorf("presence get states batch: %w", err)
	}

	for i, cmd := range cmds {
		userID := userIDs[i]
		hash, err := cmd.Result()
		if err != nil {
			return nil, fmt.Errorf("presence get state %s: %w", userID, err)
		}
		state, err := parseStateHash(userID, hash)
		if err != nil {
			return nil, err
		}
		result[userID] = state
	}
	return result, nil
}

func (r *RedisRepository) RequeueDeadline(ctx context.Context, userID uuid.UUID, dueAt time.Time) error {
	if err := r.client.ZAdd(ctx, RedisDeadlineKey, goredis.Z{Score: float64(dueAt.UTC().UnixMilli()), Member: userID.String()}).Err(); err != nil {
		return fmt.Errorf("presence requeue deadline: %w", err)
	}
	return nil
}

func parseLeaseResult(userID uuid.UUID, raw any) (*LeaseResult, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) < 6 {
		return nil, fmt.Errorf("presence lease result malformed for %s", userID)
	}
	activeCount := toInt64(values[0])
	online := toBool(values[1])
	transitioned := toBool(values[2])
	version := toInt64(values[3])
	nextDeadline := toNullableTime(values[4])
	lastSeen := toNullableTime(values[5])
	return &LeaseResult{
		UserID:           userID,
		ActiveLeaseCount: activeCount,
		IsOnline:         online,
		Transitioned:     transitioned,
		Version:          version,
		NextDeadline:     nextDeadline,
		LastSeenAt:       lastSeen,
		State: State{
			UserID:     userID,
			IsOnline:   online,
			LastSeenAt: lastSeen,
			Version:    version,
		},
	}, nil
}

func parseVersionResult(userID uuid.UUID, raw any) (*LeaseResult, error) {
	values, ok := raw.([]interface{})
	if !ok || len(values) < 2 {
		return nil, fmt.Errorf("presence version result malformed for %s", userID)
	}
	online := toBool(values[0])
	version := toInt64(values[1])
	return &LeaseResult{
		UserID:   userID,
		IsOnline: online,
		Version:  version,
		State: State{
			UserID:   userID,
			IsOnline: online,
			Version:  version,
		},
	}, nil
}

func parseStateHash(userID uuid.UUID, hash map[string]string) (State, error) {
	if len(hash) == 0 {
		return State{UserID: userID, IsOnline: false, Version: 0}, nil
	}
	onlineStr, okOnline := hash["online"]
	versionStr, okVersion := hash["version"]
	if !okOnline || !okVersion {
		return State{}, fmt.Errorf("presence malformed state for %s", userID)
	}
	online, err := strconv.ParseBool(onlineStr)
	if err != nil {
		return State{}, fmt.Errorf("presence malformed online state for %s: %w", userID, err)
	}
	version, err := strconv.ParseInt(versionStr, 10, 64)
	if err != nil {
		return State{}, fmt.Errorf("presence malformed version for %s: %w", userID, err)
	}
	return State{UserID: userID, IsOnline: online, Version: version}, nil
}

func toBool(v any) bool {
	switch t := v.(type) {
	case int64:
		return t != 0
	case int:
		return t != 0
	case string:
		return t == "1" || strings.EqualFold(t, "true")
	case []byte:
		return string(t) == "1" || strings.EqualFold(string(t), "true")
	default:
		return false
	}
}

func toInt64(v any) int64 {
	switch t := v.(type) {
	case int64:
		return t
	case int:
		return int64(t)
	case float64:
		return int64(t)
	case string:
		n, _ := strconv.ParseInt(t, 10, 64)
		return n
	case []byte:
		n, _ := strconv.ParseInt(string(t), 10, 64)
		return n
	default:
		return 0
	}
}

func toNullableTime(v any) *time.Time {
	if v == nil {
		return nil
	}
	ms := toInt64(v)
	if ms <= 0 {
		return nil
	}
	t := time.UnixMilli(ms).UTC()
	return &t
}
