package domain

import (
	"context"
	"time"
)

// Task bizim ana varlığımız (Entity). 
// Veritabanı modellerinden (sqlc) bağımsızdır.
type Task struct {
	ID        int32
	Title     string
	Done      bool
	CreatedAt time.Time
}

// TaskRepository veriye erişim kurallarını belirler.
type TaskRepository interface {
	Create(ctx context.Context, title string) (Task, error)
	GetByID(ctx context.Context, id int32) (Task, error)
	List(ctx context.Context) ([]Task, error)
	MarkDone(ctx context.Context, id int32) (Task, error)
	Delete(ctx context.Context, id int32) error
}

// TaskUsecase iş mantığı kurallarını belirler.
type TaskUsecase interface {
	CreateTask(ctx context.Context, title string) (Task, error)
	GetTask(ctx context.Context, id int32) (Task, error)
	ListTasks(ctx context.Context) ([]Task, error)
	MarkTaskDone(ctx context.Context, id int32) (Task, error)
	DeleteTask(ctx context.Context, id int32) error
}
