package cache

import (
	"hash/fnv"
	"sync"
	"time"
)

type shard struct {
	mu   sync.RWMutex
	data map[string]expiring
}

type ShardedCache struct {
	shards []shard
	ttl    time.Duration
	now    func() time.Time

	ticker *time.Ticker
	stop   chan struct{}
}

type ShardedOption func(*ShardedCache)

func WithShards(n int) ShardedOption {
	return func(c *ShardedCache) {
		if n <= 0 {
			n = 16
		}
		c.shards = make([]shard, n)
		for i := range c.shards {
			c.shards[i] = shard{data: make(map[string]expiring)}
		}
	}
}
func WithShardTTL(ttl time.Duration) ShardedOption { return func(c *ShardedCache) { c.ttl = ttl } }

func NewShardedCache(opts ...ShardedOption) *ShardedCache {
	c := &ShardedCache{now: time.Now, stop: make(chan struct{})}
	WithShards(16)(c) // default 16
	for _, o := range opts {
		o(c)
	}
	if c.ttl > 0 {
		c.ticker = time.NewTicker(c.ttl / 2)
		go func() {
			for {
				select {
				case <-c.ticker.C:
					c.purge()
				case <-c.stop:
					return
				}
			}
		}()
	}
	return c
}
func (c *ShardedCache) Close() {
	if c.ticker != nil {
		c.ticker.Stop()
	}
	close(c.stop)
}

func (c *ShardedCache) shardFor(key string) *shard {
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	idx := int(h.Sum32()) & (len(c.shards) - 1) // len — степень двойки → быстрый mod
	return &c.shards[idx]
}

func (c *ShardedCache) Put(key string, v any) {
	s := c.shardFor(key)
	s.mu.Lock()
	defer s.mu.Unlock()
	e := expiring{V: v}
	if c.ttl > 0 {
		e.E = c.now().Add(c.ttl)
	}
	s.data[key] = e
}

func (c *ShardedCache) Get(key string) (any, bool) {
	s := c.shardFor(key)
	s.mu.RLock()
	e, ok := s.data[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !e.E.IsZero() && c.now().After(e.E) {
	
		s.mu.Lock()
		if cur, ok := s.data[key]; ok && cur.E == e.E {
			delete(s.data, key)
		}
		s.mu.Unlock()
		return nil, false
	}
	return e.V, true
}

func (c *ShardedCache) Delete(key string) {
	s := c.shardFor(key)
	s.mu.Lock()
	delete(s.data, key)
	s.mu.Unlock()
}
 
func (c *ShardedCache) Snapshot() map[string]any {
	out := make(map[string]any)
	now := c.now()
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.RLock()
		for k, e := range s.data {
			if e.E.IsZero() || now.Before(e.E) {
				out[k] = e.V
			}
		}
		s.mu.RUnlock()
	}
	return out
}

func (c *ShardedCache) purge() {
	now := c.now()
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.Lock()
		for k, e := range s.data {
			if !e.E.IsZero() && now.After(e.E) {
				delete(s.data, k)
			}
		}
		s.mu.Unlock()
	}
}
