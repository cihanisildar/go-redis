-- name: GetTask :one
SELECT * FROM tasks WHERE id = $1;

-- name: ListTasks :many
SELECT * FROM tasks ORDER BY created_at DESC;

-- name: CreateTask :one
INSERT INTO tasks (title) VALUES ($1) RETURNING *;

-- name: MarkDone :one
UPDATE tasks SET done = true WHERE id = $1 RETURNING *;

-- name: DeleteTask :exec
DELETE FROM tasks WHERE id = $1;
