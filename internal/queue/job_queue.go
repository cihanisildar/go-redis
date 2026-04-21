package queue

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

const queueKey = "queue:tasks"

type JobQueue struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *JobQueue {
	return &JobQueue{rdb: rdb}
}

// Push, yeni task ID'sini kuyruğun sonuna ekler.
// LPUSH: listenin başına ekler — yani en yeni iş en önce işlenir (LIFO).
// RPUSH kullansaydık FIFO olurdu. Hangisi doğru olduğu use case'e göre değişir.
func (q *JobQueue) Push(ctx context.Context, taskID int32) error {
	return q.rdb.LPush(ctx, queueKey, taskID).Err()
}

// Len, kuyrukta bekleyen iş sayısını döner.
func (q *JobQueue) Len(ctx context.Context) (int64, error) {
	return q.rdb.LLen(ctx, queueKey).Result()
}

// StartWorker, arka planda sürekli çalışan bir goroutine başlatır.
// BRPOP: kuyrukta iş yoksa bağlantıyı bloke eder — CPU yakmaz, busy-wait olmaz.
// timeout=0 → sonsuza kadar bekle. timeout>0 → N saniye sonra boş dön ve tekrar dene.
// ctx iptal edilince worker temiz şekilde durur.
func (q *JobQueue) StartWorker(ctx context.Context) {
	go func() {
		log.Println("[worker] başladı, kuyruk dinleniyor...")
		for {
			// BRPOP: [key, value] çifti döner ya da timeout'ta nil.
			// 5 saniyelik timeout kullanıyoruz ki ctx iptalini kontrol edebilelim.
			result, err := q.rdb.BRPop(ctx, 5*time.Second, queueKey).Result()
			if ctx.Err() != nil {
				log.Println("[worker] context iptal edildi, durdu.")
				return
			}
			if err == redis.Nil {
				// Timeout: kuyruk boş, tekrar bekle.
				continue
			}
			if err != nil {
				log.Printf("[worker] BRPOP hatası: %v\n", err)
				time.Sleep(time.Second)
				continue
			}

			// result[0] = key adı, result[1] = değer
			rawID := result[1]
			taskID, err := strconv.Atoi(rawID)
			if err != nil {
				log.Printf("[worker] geçersiz task ID: %s\n", rawID)
				continue
			}

			processJob(taskID)
		}
	}()
}

// processJob, gerçek hayatta burada email gönderme, bildirim vs. olurdu.
func processJob(taskID int) {
	log.Printf("[worker] task #%d işlendi — %s\n", taskID, fmt.Sprintf("simüle edildi"))
}
