package lock

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

var ErrNotAcquired = errors.New("lock alınamadı: başka bir işlem devam ediyor")

// releaseScript: token eşleşiyorsa sil, eşleşmiyorsa dokunma.
// Kontrol + silme iki ayrı komut olsaydı aralarına başka bir process girebilirdi.
// Lua Redis'te single-threaded çalışır, bu ikisi atomik olur.
var releaseScript = redis.NewScript(`
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	end
	return 0
`)

type Lock struct {
	rdb   *redis.Client
	key   string
	token string
	ttl   time.Duration
}

// Acquire, lock almayı dener. Lock alınamazsa ErrNotAcquired döner.
// token: bu process'e özgü rastgele değer — sadece sahibi release edebilsin diye.
// NX (Not eXists): key yoksa yaz, varsa yazma.
// PX: milisaniye cinsinden TTL — uygulama çökerse lock sonsuza kalmaz.
func Acquire(ctx context.Context, rdb *redis.Client, resource string, ttl time.Duration) (*Lock, error) {
	key := fmt.Sprintf("lock:%s", resource)
	token := uuid.NewString()

	ok, err := rdb.SetNX(ctx, key, token, ttl).Result()
	if err != nil {
		return nil, fmt.Errorf("lock acquire hatası: %w", err)
	}
	if !ok {
		return nil, ErrNotAcquired
	}

	return &Lock{rdb: rdb, key: key, token: token, ttl: ttl}, nil
}

// Release, lock'u serbest bırakır. Sadece token eşleşiyorsa siler.
func (l *Lock) Release(ctx context.Context) error {
	err := releaseScript.Run(ctx, l.rdb, []string{l.key}, l.token).Err()
	if err != nil && !errors.Is(err, redis.Nil) {
		return fmt.Errorf("lock release hatası: %w", err)
	}
	return nil
}
