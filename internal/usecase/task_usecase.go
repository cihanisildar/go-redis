package usecase

import (
	"context"
	"go_redis/internal/domain"
)

type taskUsecase struct {
	repo domain.TaskRepository
}

func NewTaskUsecase(repo domain.TaskRepository) domain.TaskUsecase {
	return &taskUsecase{
		repo: repo,
	}
}

func (u *taskUsecase) CreateTask(ctx context.Context, title string) (domain.Task, error) {
	// Buraya iş kuralları gelebilir (örn: başlık kontrolü)
	return u.repo.Create(ctx, title)
}

func (u *taskUsecase) GetTask(ctx context.Context, id int32) (domain.Task, error) {
	return u.repo.GetByID(ctx, id)
}

func (u *taskUsecase) ListTasks(ctx context.Context) ([]domain.Task, error) {
	return u.repo.List(ctx)
}

func (u *taskUsecase) MarkTaskDone(ctx context.Context, id int32) (domain.Task, error) {
	return u.repo.MarkDone(ctx, id)
}

func (u *taskUsecase) DeleteTask(ctx context.Context, id int32) error {
	return u.repo.Delete(ctx, id)
}
