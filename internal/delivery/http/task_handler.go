package http

import (
	"context"
	"encoding/json"
	"errors"
	"go_redis/internal/domain"
	"go_redis/internal/lock"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type QueueStats interface {
	Len(ctx context.Context) (int64, error)
}

type TaskHandler struct {
	usecase domain.TaskUsecase
	queue   QueueStats
}

func NewTaskHandler(u domain.TaskUsecase, q QueueStats) *TaskHandler {
	return &TaskHandler{usecase: u, queue: q}
}

// TaskResponse, Domain modelini JSON'a uygun hale getiren DTO.
type TaskResponse struct {
	ID        int32  `json:"id"`
	Title     string `json:"title"`
	Done      bool   `json:"done"`
	CreatedAt string `json:"created_at"`
}

func toResponse(t domain.Task) TaskResponse {
	return TaskResponse{
		ID:        t.ID,
		Title:     t.Title,
		Done:      t.Done,
		CreatedAt: t.CreatedAt.Format(time.RFC3339),
	}
}

func (h *TaskHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/tasks", h.TasksHandler)
	mux.HandleFunc("/tasks/", h.TaskByIDHandler)
	mux.HandleFunc("/queue/stats", h.QueueStatsHandler)
}

func (h *TaskHandler) TasksHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TaskHandler) TaskByIDHandler(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/tasks/")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "geçersiz id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTask(w, r, int32(id))
	case http.MethodPut:
		h.markDone(w, r, int32(id))
	case http.MethodDelete:
		h.deleteTask(w, r, int32(id))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *TaskHandler) createTask(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Title == "" {
		http.Error(w, "title zorunlu", http.StatusBadRequest)
		return
	}

	task, err := h.usecase.CreateTask(r.Context(), body.Title)
	if err != nil {
		http.Error(w, "sunucu hatası", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusCreated, toResponse(task))
}

func (h *TaskHandler) listTasks(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.usecase.ListTasks(r.Context())
	if err != nil {
		http.Error(w, "sunucu hatası", http.StatusInternalServerError)
		return
	}

	result := make([]TaskResponse, len(tasks))
	for i, t := range tasks {
		result[i] = toResponse(t)
	}
	h.writeJSON(w, http.StatusOK, result)
}

func (h *TaskHandler) getTask(w http.ResponseWriter, r *http.Request, id int32) {
	task, err := h.usecase.GetTask(r.Context(), id)
	if err != nil {
		http.Error(w, "task bulunamadı veya sunucu hatası", http.StatusNotFound)
		return
	}

	h.writeJSON(w, http.StatusOK, toResponse(task))
}

func (h *TaskHandler) markDone(w http.ResponseWriter, r *http.Request, id int32) {
	task, err := h.usecase.MarkTaskDone(r.Context(), id)
	if err != nil {
		if errors.Is(err, lock.ErrNotAcquired) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			w.Write([]byte(`{"error":"bu task zaten işleniyor, lütfen bekleyin"}`))
			return
		}
		http.Error(w, "sunucu hatası", http.StatusInternalServerError)
		return
	}

	h.writeJSON(w, http.StatusOK, toResponse(task))
}

func (h *TaskHandler) deleteTask(w http.ResponseWriter, r *http.Request, id int32) {
	if err := h.usecase.DeleteTask(r.Context(), id); err != nil {
		http.Error(w, "sunucu hatası", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *TaskHandler) QueueStatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	length, err := h.queue.Len(r.Context())
	if err != nil {
		http.Error(w, "sunucu hatası", http.StatusInternalServerError)
		return
	}
	h.writeJSON(w, http.StatusOK, map[string]int64{"pending": length})
}

func (h *TaskHandler) writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
