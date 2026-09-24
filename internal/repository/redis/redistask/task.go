package redistask

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/logger"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/repository/redis"
)

const (
	TaskKeyPrefix = "task:"
	ListKeyPrefix = "tasks:list:"
	ListGenKey    = "tasks:list:gen"
	CountKey      = "tasks:count"
	CountLockKey  = "tasks:count:lock"

	generationTTLFactor  = 10
	countLockTTL         = 3 * time.Second
	countLockRetry       = 10 * time.Millisecond
	countAdjustmentLimit = countLockTTL + time.Second
)

var (
	releaseCountLockScript = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
    return redis.call("DEL", KEYS[1])
end
return 0`)
	adjustCountScript = goredis.NewScript(`
if redis.call("EXISTS", KEYS[1]) == 0 then
    return false
end
local value = redis.call("INCRBY", KEYS[1], ARGV[1])
if value < 0 then
    redis.call("DEL", KEYS[1])
    return false
end
return value`)
)

type Repository interface {
	Create(ctx context.Context, task entity.Task) (entity.Task, error)
	GetByID(ctx context.Context, id int64) (entity.Task, error)
	List(ctx context.Context, req param.ListTasksRequest) ([]entity.Task, error)
	Update(ctx context.Context, req param.UpdateTaskRequest) (entity.Task, error)
	Delete(ctx context.Context, id int64) error
	Count(ctx context.Context) (int64, error)
}

type DB struct {
	next Repository
	conn *redis.DB
	ttl  time.Duration
	log  logger.Logger
}

func New(next Repository, conn *redis.DB, ttl time.Duration, log logger.Logger) *DB {
	return &DB{next: next, conn: conn, ttl: ttl, log: log}
}

func (d *DB) Create(ctx context.Context, task entity.Task) (entity.Task, error) {
	created, err := d.next.Create(ctx, task)
	if err != nil {
		return entity.Task{}, err
	}

	d.retireLists(ctx)
	d.adjustCount(ctx, 1)

	return created, nil
}

func (d *DB) GetByID(ctx context.Context, id int64) (entity.Task, error) {
	key := taskKey(id)

	var cached entity.Task
	if d.load(ctx, key, &cached) {
		return cached, nil
	}

	task, err := d.next.GetByID(ctx, id)
	if err != nil {
		return entity.Task{}, err
	}

	d.store(ctx, key, task)

	return task, nil
}

func (d *DB) List(ctx context.Context, req param.ListTasksRequest) ([]entity.Task, error) {
	key := d.listKey(ctx, req)

	var cached []entity.Task
	if key != "" && d.load(ctx, key, &cached) {
		return cached, nil
	}

	tasks, err := d.next.List(ctx, req)
	if err != nil {
		return nil, err
	}

	if key != "" {
		d.store(ctx, key, tasks)
	}

	return tasks, nil
}

func (d *DB) Update(ctx context.Context, req param.UpdateTaskRequest) (entity.Task, error) {
	updated, err := d.next.Update(ctx, req)
	if err != nil {
		return entity.Task{}, err
	}

	d.evict(ctx, req.ID)
	d.retireLists(ctx)

	return updated, nil
}

func (d *DB) Delete(ctx context.Context, id int64) error {
	if err := d.next.Delete(ctx, id); err != nil {
		return err
	}

	d.evict(ctx, id)
	d.retireLists(ctx)
	d.adjustCount(ctx, -1)

	return nil
}

func (d *DB) Count(ctx context.Context) (int64, error) {
	count, err := d.conn.Client().Get(ctx, CountKey).Int64()
	if err == nil {
		return count, nil
	}
	if !errors.Is(err, goredis.Nil) {
		d.log.Warn(ctx, "cache operation failed", "operation", "get_count", "error", err)

		return d.next.Count(ctx)
	}

	token, err := d.acquireCountLock(ctx)
	if err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "acquire_count_lock", "error", err)

		return d.next.Count(ctx)
	}
	defer d.releaseCountLock(ctx, token)

	count, err = d.conn.Client().Get(ctx, CountKey).Int64()
	if err == nil {
		return count, nil
	}
	if !errors.Is(err, goredis.Nil) {
		d.log.Warn(ctx, "cache operation failed", "operation", "recheck_count", "error", err)

		return d.next.Count(ctx)
	}

	count, err = d.next.Count(ctx)
	if err != nil {
		return 0, err
	}

	if err := d.conn.Client().Set(ctx, CountKey, count, d.ttl).Err(); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "set_count", "error", err)
	}

	return count, nil
}

func (d *DB) adjustCount(ctx context.Context, delta int64) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), countAdjustmentLimit)
	defer cancel()

	token, err := d.acquireCountLock(ctx)
	if err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "acquire_count_lock", "error", err)

		return
	}
	defer d.releaseCountLock(ctx, token)

	if _, err := adjustCountScript.Run(ctx, d.conn.Client(), []string{CountKey}, delta).Result(); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "adjust_count", "error", err)
	}
}

func (d *DB) acquireCountLock(ctx context.Context) (string, error) {
	token := rand.Text()
	ticker := time.NewTicker(countLockRetry)
	defer ticker.Stop()

	for {
		acquired, err := d.conn.Client().SetNX(ctx, CountLockKey, token, countLockTTL).Result()
		if err != nil {
			return "", err
		}
		if acquired {
			return token, nil
		}

		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func (d *DB) releaseCountLock(ctx context.Context, token string) {
	if _, err := releaseCountLockScript.Run(context.WithoutCancel(ctx), d.conn.Client(), []string{CountLockKey}, token).Result(); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "release_count_lock", "error", err)
	}
}

func (d *DB) listKey(ctx context.Context, req param.ListTasksRequest) string {
	generation, err := d.conn.Client().Get(ctx, ListGenKey).Int64()
	if err != nil && !errors.Is(err, goredis.Nil) {
		d.log.Warn(ctx, "cache operation failed", "operation", "get_list_generation", "error", err)

		return ""
	}

	return fmt.Sprintf("%s%d:%s:%s:%d:%d",
		ListKeyPrefix, generation, req.Status, url.QueryEscape(req.Assignee), req.Cursor, req.Limit)
}

func (d *DB) load(ctx context.Context, key string, into any) bool {
	payload, err := d.conn.Client().Get(ctx, key).Bytes()
	if err != nil {
		if !errors.Is(err, goredis.Nil) {
			d.log.Warn(ctx, "cache operation failed", "operation", "get", "cache_key", key, "error", err)
		}

		return false
	}

	if err := json.Unmarshal(payload, into); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "decode", "cache_key", key, "error", err)

		return false
	}

	return true
}

func (d *DB) store(ctx context.Context, key string, value any) {
	payload, err := json.Marshal(value)
	if err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "encode", "cache_key", key, "error", err)

		return
	}

	if err := d.conn.Client().Set(context.WithoutCancel(ctx), key, payload, d.ttl).Err(); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "set", "cache_key", key, "error", err)
	}
}

func (d *DB) evict(ctx context.Context, id int64) {
	key := taskKey(id)
	if err := d.conn.Client().Del(context.WithoutCancel(ctx), key).Err(); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "delete", "cache_key", key, "error", err)
	}
}

func (d *DB) retireLists(ctx context.Context) {
	ctx = context.WithoutCancel(ctx)

	pipe := d.conn.Client().TxPipeline()
	pipe.Incr(ctx, ListGenKey)
	pipe.Expire(ctx, ListGenKey, d.ttl*generationTTLFactor)
	if _, err := pipe.Exec(ctx); err != nil {
		d.log.Warn(ctx, "cache operation failed", "operation", "retire_lists", "error", err)
	}
}

func taskKey(id int64) string {
	return TaskKeyPrefix + strconv.FormatInt(id, 10)
}
