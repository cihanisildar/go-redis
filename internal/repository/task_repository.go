package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"go_redis/internal/db"
	"go_redis/internal/domain"
	"go_redis/internal/lock"
	"go_redis/internal/queue"

	"github.com/redis/go-redis/v9"
)

type taskRepository struct {
	queries *db.Queries
	rdb     *redis.Client
	queue   *queue.JobQueue
}

// NewTaskRepository, domain.TaskRepository arayüzünü uygulayan bir nesne döner.
func NewTaskRepository(queries *db.Queries, rdb *redis.Client, q *queue.JobQueue) domain.TaskRepository {
	return &taskRepository{
		queries: queries,
		rdb:     rdb,
		queue:   q,
	}
}

func (r *taskRepository) Create(ctx context.Context, title string) (domain.Task, error) {
	t, err := r.queries.CreateTask(ctx, title)
	if err != nil {
		return domain.Task{}, err
	}
	domainTask := r.toDomain(t)
	// Hata olsa bile task yaratıldı — kuyruğa atamazsak loglayıp geç.
	if err := r.queue.Push(ctx, domainTask.ID); err != nil {
		fmt.Printf("queue push hatası task #%d: %v\n", domainTask.ID, err)
	}
	return domainTask, nil
}

func (r *taskRepository) GetByID(ctx context.Context, id int32) (domain.Task, error) {
	cacheKey := fmt.Sprintf("task:%d", id)

	// 1. Cache Kontrolü
	val, err := r.rdb.Get(ctx, cacheKey).Result()
	if err == nil {
		var t domain.Task
		if err := json.Unmarshal([]byte(val), &t); err == nil {
			return t, nil
		}
	}

	// 2. Veritabanı Kontrolü
	t, err := r.queries.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}

	domainTask := r.toDomain(t)

	// 3. Cache'e Yazma
	data, _ := json.Marshal(domainTask)
	r.rdb.Set(ctx, cacheKey, string(data), time.Minute)

	return domainTask, nil
}

func (r *taskRepository) List(ctx context.Context) ([]domain.Task, error) {
	tasks, err := r.queries.ListTasks(ctx)
	if err != nil {
		return nil, err
	}

	var domainTasks []domain.Task
	for _, t := range tasks {
		domainTasks = append(domainTasks, r.toDomain(t))
	}
	return domainTasks, nil
}

func (r *taskRepository) MarkDone(ctx context.Context, id int32) (domain.Task, error) {
	l, err := lock.Acquire(ctx, r.rdb, fmt.Sprintf("task:%d", id), 5*time.Second)
	if err != nil {
		return domain.Task{}, err
	}
	defer l.Release(ctx)

	t, err := r.queries.MarkDone(ctx, id)
	if err != nil {
		return domain.Task{}, err
	}

	domainTask := r.toDomain(t)
	r.rdb.Del(ctx, fmt.Sprintf("task:%d", id))

	return domainTask, nil
}

func (r *taskRepository) Delete(ctx context.Context, id int32) error {
	err := r.queries.DeleteTask(ctx, id)
	if err != nil {
		return err
	}
	// Invalidate cache
	r.rdb.Del(ctx, fmt.Sprintf("task:%d", id))
	return nil
}

// toDomain, SQLC modelini Domain modeline çevirir.
func (r *taskRepository) toDomain(t db.Task) domain.Task {
	return domain.Task{
		ID:        t.ID,
		Title:     t.Title,
		Done:      t.Done,
		CreatedAt: t.CreatedAt.Time,
	}
}
