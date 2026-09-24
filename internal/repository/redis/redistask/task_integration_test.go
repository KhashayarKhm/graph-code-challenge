//go:build integration

package redistask_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync/atomic"
	"testing"
	"time"

	goredis "github.com/redis/go-redis/v9"

	"graph-code-challenge/internal/config"
	"graph-code-challenge/internal/entity"
	"graph-code-challenge/internal/param"
	"graph-code-challenge/internal/repository/redis"
	"graph-code-challenge/internal/repository/redis/redistask"
	"graph-code-challenge/internal/service/taskservice"
)

var _ taskservice.Repository = (*redistask.DB)(nil)

const cacheTTL = time.Minute

var (
	sharedConn *redis.DB
	seq        atomic.Int64
)

func TestMain(m *testing.M) {
	code, err := runTests(m)
	if err != nil {
		fmt.Fprintln(os.Stderr, "integration setup:", err)
		os.Exit(1)
	}

	os.Exit(code)
}

func runTests(m *testing.M) (int, error) {
	cfg, err := config.Load()
	if err != nil {
		return 0, err
	}

	if cfg.RedisURL == "" {
		return 0, errors.New("REDIS_URL is empty (use ENV_FILE=.env.test, or make test-integration)")
	}

	if err := requireTestRedisDB(cfg.RedisURL); err != nil {
		return 0, err
	}

	conn, err := redis.New(context.Background(), cfg.RedisURL)
	if err != nil {
		return 0, err
	}
	defer conn.Close()

	if err := conn.Client().FlushDB(context.Background()).Err(); err != nil {
		return 0, fmt.Errorf("flushing the test cache: %w", err)
	}

	sharedConn = conn

	return m.Run(), nil
}

func requireTestRedisDB(rawURL string) error {
	options, err := goredis.ParseURL(rawURL)
	if err != nil {
		return fmt.Errorf("REDIS_URL is not a valid URL: %w", err)
	}

	if options.DB == 0 {
		return errors.New("refusing to run integration tests against Redis database 0: these tests flush the database, so point REDIS_URL at a dedicated index (use ENV_FILE=.env.test, or make test-integration)")
	}

	return nil
}

type stubRepo struct {
	task          entity.Task
	tasks         []entity.Task
	err           error
	createStarted chan struct{}
	countStarted  chan struct{}
	countRelease  chan struct{}

	createCalls int
	getCalls    int
	listCalls   int
	updateCalls int
	deleteCalls int
	countCalls  int
}

type loggerStub struct{}

func (loggerStub) Debug(context.Context, string, ...any) {}
func (loggerStub) Info(context.Context, string, ...any)  {}
func (loggerStub) Warn(context.Context, string, ...any)  {}
func (loggerStub) Error(context.Context, string, ...any) {}

func (s *stubRepo) Create(_ context.Context, _ entity.Task) (entity.Task, error) {
	s.createCalls++
	if s.createStarted != nil {
		close(s.createStarted)
	}

	return s.task, s.err
}

func (s *stubRepo) GetByID(_ context.Context, _ int64) (entity.Task, error) {
	s.getCalls++

	return s.task, s.err
}

func (s *stubRepo) List(_ context.Context, _ param.ListTasksRequest) ([]entity.Task, error) {
	s.listCalls++

	return s.tasks, s.err
}

func (s *stubRepo) Update(_ context.Context, _ param.UpdateTaskRequest) (entity.Task, error) {
	s.updateCalls++

	return s.task, s.err
}

func (s *stubRepo) Delete(_ context.Context, _ int64) error {
	s.deleteCalls++

	return s.err
}

func (s *stubRepo) Count(_ context.Context) (int64, error) {
	s.countCalls++
	if s.countStarted != nil {
		close(s.countStarted)
		<-s.countRelease
	}

	return int64(len(s.tasks)), s.err
}

func newCache(t *testing.T, stub *stubRepo) (*redistask.DB, int64) {
	t.Helper()

	id := seq.Add(1)

	t.Cleanup(func() {
		sharedConn.Client().Del(
			context.Background(),
			fmt.Sprintf("%s%d", redistask.TaskKeyPrefix, id),
			redistask.CountKey,
			redistask.CountLockKey,
		)
	})

	return redistask.New(stub, sharedConn, cacheTTL, loggerStub{}), id
}

func newListRequest() param.ListTasksRequest {
	return param.ListTasksRequest{Assignee: fmt.Sprintf("it-%d", seq.Add(1)), Limit: 20}
}

func TestGetByIDServesTheSecondReadFromTheCache(t *testing.T) {
	stub := &stubRepo{task: entity.Task{ID: 7, Title: "cached", Status: entity.TaskStatusPending}}
	repo, id := newCache(t, stub)

	first, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("first get: %v", err)
	}

	second, err := repo.GetByID(context.Background(), id)
	if err != nil {
		t.Fatalf("second get: %v", err)
	}

	if stub.getCalls != 1 {
		t.Errorf("repository reads = %d, want 1", stub.getCalls)
	}

	if first != second {
		t.Errorf("cached task = %+v, want %+v", second, first)
	}
}

func TestGetByIDDoesNotCacheRepositoryErrors(t *testing.T) {
	stub := &stubRepo{err: entity.ErrTaskNotFound}
	repo, id := newCache(t, stub)

	for range 2 {
		if _, err := repo.GetByID(context.Background(), id); !errors.Is(err, entity.ErrTaskNotFound) {
			t.Fatalf("error = %v, want ErrTaskNotFound", err)
		}
	}

	if stub.getCalls != 2 {
		t.Errorf("repository reads = %d, want 2 (misses must not be cached)", stub.getCalls)
	}
}

func TestUpdateEvictsTheCachedTask(t *testing.T) {
	stub := &stubRepo{task: entity.Task{ID: 7, Title: "before"}}
	repo, id := newCache(t, stub)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("warming the cache: %v", err)
	}

	title := "after"
	if _, err := repo.Update(context.Background(), param.UpdateTaskRequest{ID: id, Title: &title}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("get after update: %v", err)
	}

	if stub.getCalls != 2 {
		t.Errorf("repository reads = %d, want 2 (update must evict task:%d)", stub.getCalls, id)
	}
}

func TestFailedUpdateKeepsTheCachedTask(t *testing.T) {
	stub := &stubRepo{task: entity.Task{ID: 7, Title: "before"}}
	repo, id := newCache(t, stub)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("warming the cache: %v", err)
	}

	stub.err = errors.New("write failed")

	if _, err := repo.Update(context.Background(), param.UpdateTaskRequest{ID: id}); err == nil {
		t.Fatal("update error = nil, want the repository error")
	}

	stub.err = nil

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("get after failed update: %v", err)
	}

	if stub.getCalls != 1 {
		t.Errorf("repository reads = %d, want 1 (a failed write must not evict)", stub.getCalls)
	}
}

func TestDeleteEvictsTheCachedTask(t *testing.T) {
	stub := &stubRepo{task: entity.Task{ID: 7, Title: "doomed"}}
	repo, id := newCache(t, stub)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("warming the cache: %v", err)
	}

	if err := repo.Delete(context.Background(), id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("get after delete: %v", err)
	}

	if stub.getCalls != 2 {
		t.Errorf("repository reads = %d, want 2 (delete must evict task:%d)", stub.getCalls, id)
	}
}

func TestListServesTheSecondReadFromTheCache(t *testing.T) {
	stub := &stubRepo{tasks: []entity.Task{{ID: 1, Title: "a"}, {ID: 2, Title: "b"}}}
	repo, _ := newCache(t, stub)
	req := newListRequest()

	first, err := repo.List(context.Background(), req)
	if err != nil {
		t.Fatalf("first list: %v", err)
	}

	second, err := repo.List(context.Background(), req)
	if err != nil {
		t.Fatalf("second list: %v", err)
	}

	if stub.listCalls != 1 {
		t.Errorf("repository lists = %d, want 1", stub.listCalls)
	}

	if len(second) != len(first) || second[0] != first[0] || second[1] != first[1] {
		t.Errorf("cached page = %+v, want %+v", second, first)
	}
}

func TestListKeysAreFilterSpecific(t *testing.T) {
	stub := &stubRepo{tasks: []entity.Task{{ID: 1}}}
	repo, _ := newCache(t, stub)
	req := newListRequest()

	pending := req
	pending.Status = entity.TaskStatusPending

	done := req
	done.Status = entity.TaskStatusDone

	for _, filtered := range []param.ListTasksRequest{pending, done, pending, done} {
		if _, err := repo.List(context.Background(), filtered); err != nil {
			t.Fatalf("list %s: %v", filtered.Status, err)
		}
	}

	if stub.listCalls != 2 {
		t.Errorf("repository lists = %d, want 2 (one per distinct filter)", stub.listCalls)
	}
}

func TestWritesRetireEveryCachedList(t *testing.T) {
	testCases := []struct {
		name  string
		write func(repo *redistask.DB) error
	}{
		{"create", func(repo *redistask.DB) error {
			_, err := repo.Create(context.Background(), entity.Task{Title: "new"})

			return err
		}},
		{"update", func(repo *redistask.DB) error {
			_, err := repo.Update(context.Background(), param.UpdateTaskRequest{ID: 1})

			return err
		}},
		{"delete", func(repo *redistask.DB) error {
			return repo.Delete(context.Background(), 1)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stub := &stubRepo{tasks: []entity.Task{{ID: 1}}}
			repo, _ := newCache(t, stub)
			req := newListRequest()

			if _, err := repo.List(context.Background(), req); err != nil {
				t.Fatalf("warming the cache: %v", err)
			}

			if err := testCase.write(repo); err != nil {
				t.Fatalf("%s: %v", testCase.name, err)
			}

			if _, err := repo.List(context.Background(), req); err != nil {
				t.Fatalf("list after %s: %v", testCase.name, err)
			}

			if stub.listCalls != 2 {
				t.Errorf("repository lists = %d, want 2 (%s must bump the list generation)", stub.listCalls, testCase.name)
			}
		})
	}
}

func TestFailedWriteDoesNotRetireCachedLists(t *testing.T) {
	stub := &stubRepo{tasks: []entity.Task{{ID: 1}}}
	repo, _ := newCache(t, stub)
	req := newListRequest()

	if _, err := repo.List(context.Background(), req); err != nil {
		t.Fatalf("warming the cache: %v", err)
	}

	stub.err = errors.New("write failed")

	if _, err := repo.Create(context.Background(), entity.Task{Title: "doomed"}); err == nil {
		t.Fatal("create error = nil, want the repository error")
	}

	stub.err = nil

	if _, err := repo.List(context.Background(), req); err != nil {
		t.Fatalf("list after failed create: %v", err)
	}

	if stub.listCalls != 1 {
		t.Errorf("repository lists = %d, want 1 (a failed write must not bump the generation)", stub.listCalls)
	}
}

func TestEveryCacheKeyExpires(t *testing.T) {
	stub := &stubRepo{task: entity.Task{ID: 7}, tasks: []entity.Task{{ID: 7}}}
	repo, id := newCache(t, stub)

	if _, err := repo.GetByID(context.Background(), id); err != nil {
		t.Fatalf("get: %v", err)
	}

	if _, err := repo.List(context.Background(), newListRequest()); err != nil {
		t.Fatalf("list: %v", err)
	}

	if _, err := repo.Create(context.Background(), entity.Task{Title: "new"}); err != nil {
		t.Fatalf("create: %v", err)
	}

	keys, err := sharedConn.Client().Keys(context.Background(), "*").Result()
	if err != nil {
		t.Fatalf("listing keys: %v", err)
	}

	if len(keys) == 0 {
		t.Fatal("no cache keys were written")
	}

	for _, key := range keys {
		ttl, err := sharedConn.Client().TTL(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("ttl of %q: %v", key, err)
		}

		if ttl <= 0 {
			t.Errorf("key %q has ttl %s, want a positive expiry", key, ttl)
		}
	}
}

func TestCountServesTheSecondReadFromTheCache(t *testing.T) {
	stub := &stubRepo{tasks: []entity.Task{{ID: 1}}}
	repo, _ := newCache(t, stub)

	for range 2 {
		if _, err := repo.Count(context.Background()); err != nil {
			t.Fatalf("count: %v", err)
		}
	}

	if stub.countCalls != 1 {
		t.Errorf("repository counts = %d, want 1", stub.countCalls)
	}

	ttl, err := sharedConn.Client().TTL(context.Background(), redistask.CountKey).Result()
	if err != nil {
		t.Fatalf("count key ttl: %v", err)
	}
	if ttl <= 0 {
		t.Errorf("count key ttl = %s, want a positive expiry", ttl)
	}
}

func TestCreateAndDeleteAdjustCachedCount(t *testing.T) {
	stub := &stubRepo{tasks: []entity.Task{{ID: 1}}}
	repo, _ := newCache(t, stub)

	if count, err := repo.Count(context.Background()); err != nil || count != 1 {
		t.Fatalf("initial count = %d, %v; want 1, nil", count, err)
	}

	if _, err := repo.Create(context.Background(), entity.Task{Title: "new"}); err != nil {
		t.Fatalf("create: %v", err)
	}
	if count, err := repo.Count(context.Background()); err != nil || count != 2 {
		t.Fatalf("count after create = %d, %v; want 2, nil", count, err)
	}

	if err := repo.Delete(context.Background(), 1); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if count, err := repo.Count(context.Background()); err != nil || count != 1 {
		t.Fatalf("count after delete = %d, %v; want 1, nil", count, err)
	}

	if stub.countCalls != 1 {
		t.Errorf("repository counts = %d, want 1", stub.countCalls)
	}
}

func TestCountInitializationDoesNotOverwriteConcurrentCreate(t *testing.T) {
	stub := &stubRepo{
		tasks:         []entity.Task{{ID: 1}},
		createStarted: make(chan struct{}),
		countStarted:  make(chan struct{}),
		countRelease:  make(chan struct{}),
	}
	repo, _ := newCache(t, stub)

	countDone := make(chan error, 1)
	go func() {
		_, err := repo.Count(context.Background())
		countDone <- err
	}()

	<-stub.countStarted

	createDone := make(chan error, 1)
	go func() {
		_, err := repo.Create(context.Background(), entity.Task{Title: "new"})
		createDone <- err
	}()

	<-stub.createStarted
	close(stub.countRelease)

	if err := <-countDone; err != nil {
		t.Fatalf("initial count: %v", err)
	}
	if err := <-createDone; err != nil {
		t.Fatalf("concurrent create: %v", err)
	}

	count, err := repo.Count(context.Background())
	if err != nil {
		t.Fatalf("final count: %v", err)
	}
	if count != 2 {
		t.Errorf("final count = %d, want 2", count)
	}
}
