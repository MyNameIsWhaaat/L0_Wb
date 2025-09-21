package cache

import (
	"sync"
	"time"
)

type KV interface {
	Put(key string, v any)
	Get(key string) (any, bool)
	Delete(key string)
	Snapshot() map[string]any
}

type Cache struct {
	Data  map[string]any
	Mutex sync.RWMutex

	ttl    time.Duration
	ticker *time.Ticker
	stop   chan struct{}
	now    func() time.Time
}

type Option func(*Cache)

func WithTTL(ttl time.Duration) Option { return func(c *Cache) { c.ttl = ttl } }
func WithNoJanitor() Option            { return func(c *Cache) { c.ticker = nil } }

func NewCache(opts ...Option) *Cache {
	c := &Cache{
		Data: make(map[string]any),
		ttl:  0,
		stop: make(chan struct{}),
		now:  time.Now,
	}
	for _, o := range opts {
		o(c)
	}

	if c.ttl > 0 {
		c.ticker = time.NewTicker(c.ttl / 2)
		go func() {
			for {
				select {
				case <-c.ticker.C:
					c.purgeExpired()
				case <-c.stop:
					return
				}
			}
		}()
	}
	return c
}

func (c *Cache) Close() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	close(c.stop)
}

type expiring struct {
	V any
	E time.Time
}

func (c *Cache) Put(key string, v any) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.ttl > 0 {
		c.Data[key] = expiring{V: v, E: c.now().Add(c.ttl)}
	} else {
		c.Data[key] = expiring{V: v}
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.Mutex.RLock()
	val, ok := c.Data[key]
	c.Mutex.RUnlock()
	if !ok {
		return nil, false
	}
	if ex, ok := val.(expiring); ok {
		if !ex.E.IsZero() && c.now().After(ex.E) {
			c.Delete(key)
			return nil, false
		}
		return ex.V, true
	}
	return val, true
}

func (c *Cache) Delete(key string) {
	c.Mutex.Lock()
	delete(c.Data, key)
	c.Mutex.Unlock()
}

func (c *Cache) purgeExpired() {
	now := c.now()
	c.Mutex.Lock()
	for k, v := range c.Data {
		if ex, ok := v.(expiring); ok && !ex.E.IsZero() && now.After(ex.E) {
			delete(c.Data, k)
		}
	}
	c.Mutex.Unlock()
}

func (c *Cache) Snapshot() map[string]any {
	c.Mutex.RLock()
	defer c.Mutex.RUnlock()

	out := make(map[string]any, len(c.Data))
	now := c.now()

	for k, v := range c.Data {
		if ex, ok := v.(expiring); ok {
			if !ex.E.IsZero() && now.After(ex.E) {
				continue
			}
			out[k] = ex.V
			continue
		}
		out[k] = v
	}
	return out
}
