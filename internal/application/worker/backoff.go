package worker

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

// Backoff computes exponential retry delays with optional jitter.
type Backoff struct {
	Base time.Duration
	Max  time.Duration
	// Float63 returns a value in [0,1). When nil, a local rand source is used.
	Float63 func() float64
}

type jitterSource struct {
	mu  sync.Mutex
	rng *rand.Rand
}

func newJitterSource(seed int64) *jitterSource {
	return &jitterSource{rng: rand.New(rand.NewSource(seed))}
}

func (s *jitterSource) Float63() float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.rng.Float64()
}

// Delay returns the delay before the next attempt.
// attemptCount is the post-claim attempt number (1 after first claim failure).
func (b Backoff) Delay(attemptCount int) time.Duration {
	base := b.Base
	max := b.Max
	if base <= 0 {
		base = 5 * time.Second
	}
	if max < base {
		max = base
	}
	if attemptCount < 1 {
		attemptCount = 1
	}

	// exp = base * 2^(attemptCount-1), capped
	exp := float64(base)
	shift := attemptCount - 1
	for i := 0; i < shift; i++ {
		if exp >= float64(max) || exp > float64(math.MaxInt64)/2 {
			exp = float64(max)
			break
		}
		exp *= 2
	}
	if exp > float64(max) {
		exp = float64(max)
	}

	f := b.Float63
	if f == nil {
		f = newJitterSource(time.Now().UnixNano()).Float63
	}
	j := f()
	if j < 0 {
		j = 0
	}
	if j > 1 {
		j = 1
	}
	// Full jitter in [0.5, 1.0] of the exponential delay.
	jitter := 0.5 + 0.5*j
	d := time.Duration(exp * jitter)
	if d < 0 || d > max {
		return max
	}
	if d < base/2 {
		// Keep a sensible floor for tiny floating results.
		half := base / 2
		if half <= 0 {
			return base
		}
		return half
	}
	return d
}
