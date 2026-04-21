package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"go_redis/internal/db"
	delivery "go_redis/internal/delivery/http"
	"go_redis/internal/middleware"
	"go_redis/internal/queue"
	"go_redis/internal/repository"
	"go_redis/internal/usecase"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	ctx := context.Background()

	// 1. Postgres Bağlantısı
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal("Postgres bağlantısı kurulamadı:", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("Postgres ping başarısız:", err)
	}
	fmt.Println("Postgres'e bağlandı!")

	// sqlc queries
	queries := db.New(pool)

	// 2. Redis Bağlantısı
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if _, err := rdb.Ping(ctx).Result(); err != nil {
		log.Fatal("Redis bağlantısı kurulamadı:", err)
	}
	fmt.Println("Redis'e bağlandı!")

	// 3. Katmanların Birbirine Bağlanması (Dependency Injection)
	q := queue.New(rdb)
	q.StartWorker(ctx)

	repo := repository.NewTaskRepository(queries, rdb, q)
	uc := usecase.NewTaskUsecase(repo)
	handler := delivery.NewTaskHandler(uc, q)

	// 4. Routing
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// 5. Middleware: dakikada 10 istek / IP
	limiter := middleware.NewRateLimiter(rdb, 10, time.Minute)

	// 6. Sunucunun Başlatılması
	fmt.Println("Sunucu Clean Architecture yapısında :8080 adresinde çalışıyor...")
	log.Fatal(http.ListenAndServe(":8080", limiter.Middleware(mux)))
}
