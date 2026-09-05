package riotapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const quotaPrefix = "gogg:riot-quota:v1:"

const quotaWeightKey = quotaPrefix + "weights"

var acquireQuotaScript = redis.NewScript(`
local dimensions = tonumber(ARGV[1])
local windows = tonumber(ARGV[2])
for i = 1, dimensions do
  local blocked = redis.call('PTTL', KEYS[i] .. ':blocked')
  if blocked > 0 then return blocked end
end
for i = 1, windows do
  local cap = tonumber(ARGV[2 + ((i - 1) * 2) + 1])
  local period = tonumber(ARGV[2 + ((i - 1) * 2) + 2])
  local key = KEYS[dimensions + i] .. ':w:' .. period
  local current = tonumber(redis.call('GET', key) or '0')
  if current >= cap then
    local ttl = redis.call('PTTL', key)
    if ttl < 1 then ttl = period end
    return ttl
  end
end
for i = 1, windows do
  local period = tonumber(ARGV[2 + ((i - 1) * 2) + 2])
  local key = KEYS[dimensions + i] .. ':w:' .. period
  local value = redis.call('INCR', key)
  if value == 1 then redis.call('PEXPIRE', key, period) end
end
return 0
`)

var observeCountScript = redis.NewScript(`
local current = tonumber(redis.call('GET', KEYS[1]) or '0')
local observed = tonumber(ARGV[1])
if observed > current then
  local ttl = redis.call('PTTL', KEYS[1])
  if ttl < 1 then ttl = ARGV[2] end
  redis.call('SET', KEYS[1], observed, 'PX', ttl)
end
return 1
`)

var extendCooldownScript = redis.NewScript(`
local requested = tonumber(ARGV[1])
for i = 1, #KEYS do
  local current = redis.call('PTTL', KEYS[i])
  if current < requested then redis.call('SET', KEYS[i], '1', 'PX', requested) end
end
return 1
`)

// RedisQuotaCoordinator is the cross-process Riot quota authority. It uses
// conservative fixed windows until actual Riot headers provide tighter caps.
type RedisQuotaCoordinator struct {
	client          redis.UniversalClient
	pollInterval    time.Duration
	defaultShortCap int
	defaultLongCap  int
}

func NewRedisQuotaCoordinator(client redis.UniversalClient) (*RedisQuotaCoordinator, error) {
	if client == nil {
		return nil, fmt.Errorf("nil Redis client")
	}
	return &RedisQuotaCoordinator{
		client:          client,
		pollInterval:    250 * time.Millisecond,
		defaultShortCap: 18,
		defaultLongCap:  90,
	}, nil
}

func (q *RedisQuotaCoordinator) Acquire(ctx context.Context, scope QuotaScope) error {
	dimensionKeys := quotaDimensionKeys(scope)
	blockKeys := quotaBlockKeys(scope)
	for {
		windowKeys := make([]string, 0, len(dimensionKeys)*2)
		windowArgs := make([]any, 0, len(dimensionKeys)*4)
		for _, key := range dimensionKeys {
			windows, err := q.windows(ctx, key)
			if err != nil {
				return fmt.Errorf("read distributed Riot quota: %w", err)
			}
			for _, window := range windows {
				windowKeys = append(windowKeys, key)
				windowArgs = append(windowArgs, window.Limit, window.Period.Milliseconds())
			}
		}
		appWindows, err := q.windows(ctx, dimensionKeys[0])
		if err != nil {
			return fmt.Errorf("read Riot application quota for fairness: %w", err)
		}
		fairnessKey, fairnessWindows, err := q.fairnessWindows(ctx, scope, appWindows)
		if err != nil {
			return fmt.Errorf("read distributed Riot fairness state: %w", err)
		}
		for _, window := range fairnessWindows {
			windowKeys = append(windowKeys, fairnessKey)
			windowArgs = append(windowArgs, window.Limit, window.Period.Milliseconds())
		}
		scriptKeys := append(append([]string{}, blockKeys...), windowKeys...)
		args := []any{len(blockKeys), len(windowKeys)}
		args = append(args, windowArgs...)
		waitMS, err := acquireQuotaScript.Run(ctx, q.client, scriptKeys, args...).Int64()
		if err != nil {
			return fmt.Errorf("acquire distributed Riot quota: %w", err)
		}
		if waitMS <= 0 {
			return nil
		}
		wait := time.Duration(waitMS) * time.Millisecond
		if wait > q.pollInterval {
			wait = q.pollInterval
		}
		if err := waitContext(ctx, wait); err != nil {
			return err
		}
	}
}

func (q *RedisQuotaCoordinator) fairnessWindows(ctx context.Context, scope QuotaScope, appWindows []rateWindow) (string, []rateWindow, error) {
	product := normalizedQuotaProduct(scope.Product)
	activeKeys := []string{
		quotaKey(scope, "active", scope.Credential, scope.Route, "lol"),
		quotaKey(scope, "active", scope.Credential, scope.Route, "tft"),
	}
	selfKey := quotaKey(scope, "active", scope.Credential, scope.Route, product)
	pipe := q.client.Pipeline()
	pipe.Set(ctx, selfKey, "1", 3*time.Second)
	activeCommand := pipe.MGet(ctx, activeKeys...)
	weightCommand := pipe.HMGet(ctx, quotaWeightKey, "lol", "tft")
	if _, err := pipe.Exec(ctx); err != nil {
		return "", nil, err
	}
	weights := map[string]int{"lol": 1, "tft": 1}
	for index, raw := range weightCommand.Val() {
		if parsed, err := strconv.Atoi(fmt.Sprint(raw)); err == nil && parsed > 0 {
			weights[[]string{"lol", "tft"}[index]] = parsed
		}
	}
	activeWeight := weights[product]
	for index, raw := range activeCommand.Val() {
		candidate := []string{"lol", "tft"}[index]
		if candidate != product && raw != nil {
			activeWeight += weights[candidate]
		}
	}
	share := make([]rateWindow, 0, len(appWindows))
	for _, window := range appWindows {
		share = append(share, rateWindow{
			Limit:  quotaShareLimit(window.Limit, weights[product], activeWeight),
			Period: window.Period,
		})
	}
	return quotaKey(scope, "fair", scope.Credential, scope.Route, product), share, nil
}

func quotaShareLimit(limit, productWeight, activeWeight int) int {
	if limit < 1 || productWeight < 1 || activeWeight < productWeight {
		return 1
	}
	return max(1, limit*productWeight/activeWeight)
}

func normalizedQuotaProduct(product string) string {
	if strings.EqualFold(product, "tft") {
		return "tft"
	}
	return "lol"
}

func (q *RedisQuotaCoordinator) windows(ctx context.Context, key string) ([]rateWindow, error) {
	values, err := q.client.HGetAll(ctx, key+":caps").Result()
	if err != nil {
		return nil, err
	}
	limits := make(map[int64]int)
	for periodText, limitText := range values {
		periodMS, err1 := strconv.ParseInt(periodText, 10, 64)
		limit, err2 := strconv.Atoi(limitText)
		if err1 == nil && err2 == nil && periodMS > 0 && limit > 0 {
			limits[periodMS] = limit
		}
	}
	if len(limits) == 0 {
		limits[int64(time.Second/time.Millisecond)] = q.defaultShortCap
		limits[int64((120*time.Second)/time.Millisecond)] = q.defaultLongCap
	}
	out := make([]rateWindow, 0, len(limits))
	for periodMS, limit := range limits {
		out = append(out, rateWindow{Limit: limit, Period: time.Duration(periodMS) * time.Millisecond})
	}
	return out, nil
}

func (q *RedisQuotaCoordinator) Observe(ctx context.Context, scope QuotaScope, status int, headers http.Header) error {
	appWindows := parseRateWindows(headers.Get("X-App-Rate-Limit"), headers.Get("X-App-Rate-Limit-Count"))
	methodWindows := parseRateWindows(headers.Get("X-Method-Rate-Limit"), headers.Get("X-Method-Rate-Limit-Count"))
	appKey := quotaKey(scope, "app", scope.Credential, scope.Route)
	methodKey := quotaKey(scope, "method", scope.Credential, scope.Route, scope.Method)
	if err := q.observeWindows(ctx, appKey, appWindows); err != nil {
		return err
	}
	if err := q.observeWindows(ctx, methodKey, methodWindows); err != nil {
		return err
	}
	if status != http.StatusTooManyRequests {
		return nil
	}
	retry := parseRetryAfter(headers.Get("Retry-After"), time.Now())
	if retry <= 0 {
		retry = time.Second
	}
	var keys []string
	familyKey := quotaKey(scope, "family", scope.Credential, scope.Family)
	switch strings.ToLower(headers.Get("X-Rate-Limit-Type")) {
	case "application":
		keys = []string{appKey, familyKey}
	case "method":
		keys = []string{methodKey}
	case "service":
		keys = []string{quotaKey(scope, "service", scope.Route, scope.Service)}
	default:
		keys = []string{appKey, methodKey, quotaKey(scope, "service", scope.Route, scope.Service), familyKey}
	}
	blockedKeys := make([]string, 0, len(keys))
	for _, key := range keys {
		blockedKeys = append(blockedKeys, key+":blocked")
	}
	if _, err := extendCooldownScript.Run(ctx, q.client, blockedKeys, retry.Milliseconds()).Result(); err != nil {
		return fmt.Errorf("set Riot quota cooldown: %w", err)
	}
	return nil
}

func (q *RedisQuotaCoordinator) observeWindows(ctx context.Context, key string, windows []rateWindow) error {
	if len(windows) == 0 {
		return nil
	}
	values := make(map[string]any, len(windows))
	for _, window := range windows {
		values[strconv.FormatInt(window.Period.Milliseconds(), 10)] = window.Limit
	}
	pipe := q.client.TxPipeline()
	pipe.Del(ctx, key+":caps")
	pipe.HSet(ctx, key+":caps", values)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("replace Riot quota caps: %w", err)
	}
	for _, window := range windows {
		periodMS := window.Period.Milliseconds()
		counterKey := fmt.Sprintf("%s:w:%d", key, periodMS)
		if _, err := observeCountScript.Run(ctx, q.client, []string{counterKey}, window.Count, periodMS).Result(); err != nil {
			return fmt.Errorf("store Riot quota count: %w", err)
		}
	}
	return nil
}

func quotaDimensionKeys(scope QuotaScope) []string {
	return []string{
		quotaKey(scope, "app", scope.Credential, scope.Route),
		quotaKey(scope, "method", scope.Credential, scope.Route, scope.Method),
		quotaKey(scope, "family", scope.Credential, scope.Family),
	}
}

func quotaBlockKeys(scope QuotaScope) []string {
	return append(quotaDimensionKeys(scope), quotaKey(scope, "service", scope.Route, scope.Service))
}

func quotaKey(scope QuotaScope, parts ...string) string {
	return quotaPrefix + "{" + scope.Family + "}:" + strings.Join(parts, ":")
}

var _ QuotaCoordinator = (*RedisQuotaCoordinator)(nil)
