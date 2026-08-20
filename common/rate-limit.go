package common

import (
	"sync"
	"time"
)

type InMemoryRateLimiter struct {
	store              map[string]*[]int64
	mutex              sync.Mutex
	expirationDuration time.Duration
}

func (l *InMemoryRateLimiter) Init(expirationDuration time.Duration) {
	if l.store == nil {
		l.mutex.Lock()
		if l.store == nil {
			l.store = make(map[string]*[]int64)
			l.expirationDuration = expirationDuration
			if expirationDuration > 0 {
				go l.clearExpiredItems()
			}
		}
		l.mutex.Unlock()
	}
}

func (l *InMemoryRateLimiter) clearExpiredItems() {
	for {
		time.Sleep(l.expirationDuration)
		l.mutex.Lock()
		now := time.Now().Unix()
		for key := range l.store {
			queue := l.store[key]
			size := len(*queue)
			if size == 0 || now-(*queue)[size-1] > int64(l.expirationDuration.Seconds()) {
				delete(l.store, key)
			}
		}
		l.mutex.Unlock()
	}
}

type InMemoryRateLimitReservation struct {
	limiter   *InMemoryRateLimiter
	key       string
	timestamp int64
	released  bool
	mutex     sync.Mutex
}

func (r *InMemoryRateLimitReservation) Release() {
	if r == nil || r.limiter == nil {
		return
	}
	r.mutex.Lock()
	defer r.mutex.Unlock()
	if r.released {
		return
	}
	r.limiter.mutex.Lock()
	defer r.limiter.mutex.Unlock()
	queue, ok := r.limiter.store[r.key]
	if !ok {
		r.released = true
		return
	}
	for index, timestamp := range *queue {
		if timestamp != r.timestamp {
			continue
		}
		*queue = append((*queue)[:index], (*queue)[index+1:]...)
		break
	}
	if len(*queue) == 0 {
		delete(r.limiter.store, r.key)
	}
	r.released = true
}

func (l *InMemoryRateLimiter) Reserve(key string, maxRequestNum int, duration int64) (bool, *InMemoryRateLimitReservation, int) {
	allowed, reservation, current, _, _ := l.ReserveWithWindow(key, maxRequestNum, duration)
	return allowed, reservation, current
}

// ReserveWithWindow is the RPM reservation variant that exposes the oldest
// active reservation and its absolute window end. The boundary is returned on
// rejection so review triggers can deduplicate the same limiter episode.
func (l *InMemoryRateLimiter) ReserveWithWindow(key string, maxRequestNum int, duration int64) (bool, *InMemoryRateLimitReservation, int, int64, int64) {
	if maxRequestNum <= 0 || duration <= 0 {
		return true, nil, 0, 0, 0
	}
	l.mutex.Lock()
	defer l.mutex.Unlock()
	queue, ok := l.store[key]
	now := time.Now().UnixNano()
	cutoff := now - duration*int64(time.Second)
	if !ok {
		values := make([]int64, 0, maxRequestNum)
		queue = &values
		l.store[key] = queue
	}
	firstActive := 0
	for firstActive < len(*queue) && (*queue)[firstActive] <= cutoff {
		firstActive++
	}
	if firstActive > 0 {
		*queue = append((*queue)[:0], (*queue)[firstActive:]...)
	}
	if len(*queue) >= maxRequestNum {
		oldest := (*queue)[0]
		windowStart := time.Unix(0, oldest).Unix()
		windowEnd := (oldest + duration*int64(time.Second) + int64(time.Second) - 1) / int64(time.Second)
		return false, nil, len(*queue) + 1, windowStart, windowEnd
	}
	*queue = append(*queue, now)
	return true, &InMemoryRateLimitReservation{limiter: l, key: key, timestamp: now}, len(*queue), 0, 0
}

func (l *InMemoryRateLimiter) Delete(key string) {
	l.mutex.Lock()
	delete(l.store, key)
	l.mutex.Unlock()
}

// Request parameter duration's unit is seconds
func (l *InMemoryRateLimiter) Request(key string, maxRequestNum int, duration int64) bool {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	// [old <-- new]
	queue, ok := l.store[key]
	now := time.Now().Unix()
	if ok {
		if len(*queue) < maxRequestNum {
			*queue = append(*queue, now)
			return true
		} else {
			if now-(*queue)[0] >= duration {
				*queue = (*queue)[1:]
				*queue = append(*queue, now)
				return true
			} else {
				return false
			}
		}
	} else {
		s := make([]int64, 0, maxRequestNum)
		l.store[key] = &s
		*(l.store[key]) = append(*(l.store[key]), now)
	}
	return true
}
