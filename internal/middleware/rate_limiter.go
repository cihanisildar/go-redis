package middleware

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

type RateLimiter struct {
	rdb    *redis.Client
	limit  int
	window time.Duration
}

func NewRateLimiter(rdb *redis.Client, limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{rdb: rdb, limit: limit, window: window}
}

// incrScript atomik olarak sayacı artırır ve ilk istekte TTL'i set eder.
// İki ayrı komut (INCR + EXPIRE) yerine Lua kullanmak race condition'ı önler:
// Redis, Lua scriptlerini single-threaded çalıştırır — ya ikisi olur ya hiçbiri.
var incrScript = redis.NewScript(`
	local current = redis.call('INCR', KEYS[1])
	if current == 1 then
		redis.call('EXPIRE', KEYS[1], ARGV[1])
	end
	local ttl = redis.call('TTL', KEYS[1])
	return {current, ttl}
`)

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		key := fmt.Sprintf("rl:%s", ip)
		windowSecs := int(rl.window.Seconds())

		result, err := incrScript.Run(r.Context(), rl.rdb, []string{key}, windowSecs).Result()
		if err != nil {
			// Fail open: Redis çökerse isteği engelleme, geçir.
			// Production'da burada bir alert/metric göndermek gerekir.
			next.ServeHTTP(w, r)
			return
		}

		vals := result.([]interface{})
		current := int(vals[0].(int64))
		ttl := int(vals[1].(int64))

		remaining := rl.limit - current
		if remaining < 0 {
			remaining = 0
		}

		// RFC 6585 standart rate limit başlıkları
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.Itoa(ttl))

		if current > rl.limit {
			w.Header().Set("Retry-After", strconv.Itoa(ttl))
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":"rate limit exceeded"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// clientIP, proxy arkasındaki gerçek IP'yi döner.
// X-Forwarded-For virgülle ayrılmış liste olabilir (proxy zincirleri),
// ilk eleman her zaman orijinal istemcidir.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i != -1 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
