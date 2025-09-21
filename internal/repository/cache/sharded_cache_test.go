package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestShardedCache_Default_NoTTL_PutGetDeleteSnapshot(t *testing.T) {
	c := NewShardedCache()
	defer c.Close()

	require.Equal(t, 16, len(c.shards))

	c.Put("a", 1)
	c.Put("b", "two")

	v, ok := c.Get("a")
	require.True(t, ok)
	require.Equal(t, 1, v)

	snap := c.Snapshot()
	require.Len(t, snap, 2)
	require.Equal(t, "two", snap["b"])

	c.Delete("a")
	_, ok = c.Get("a")
	require.False(t, ok)

	c2 := NewShardedCache(WithShards(0))
	require.Equal(t, 16, len(c2.shards))
	c2.Close()
}

func TestShardedCache_CustomShardCount_Distribution(t *testing.T) {
	c := NewShardedCache(WithShards(8))
	defer c.Close()

	for i := 0; i < 100; i++ {
		c.Put(fmt.Sprintf("k%d", i), i)
	}

	total := 0
	used := 0
	for i := range c.shards {
		s := &c.shards[i]
		s.mu.RLock()
		total += len(s.data)
		if len(s.data) > 0 {
			used++
		}
		s.mu.RUnlock()
	}
	require.Equal(t, 100, total)
	require.GreaterOrEqual(t, used, 2)
}

func TestShardedCache_TTL_LazyAndPurge(t *testing.T) {
	ttl := 30 * time.Millisecond
	c := NewShardedCache(WithShardTTL(ttl))
	defer c.Close()

	c.Put("x", 42)
	time.Sleep(ttl + 15*time.Millisecond)
	_, ok := c.Get("x")
	require.False(t, ok, "lazy delete on Get should remove expired key")

	c.Put("y", 99)
	time.Sleep(ttl/3)
	snapBefore := c.Snapshot()
	require.Contains(t, snapBefore, "y")

	time.Sleep(ttl + ttl/2 + 20*time.Millisecond)
	snapAfter := c.Snapshot()
	_, present := snapAfter["y"]
	require.False(t, present, "purge should remove expired key from shards")
}

func TestShardedCache_Get_Expired_TriggersLazyDelete(t *testing.T) {
	c := NewShardedCache(WithShards(4), WithShardTTL(10*time.Millisecond))
	defer c.Close()

	var clock = time.Unix(0, 0)
	c.now = func() time.Time { return clock }

	c.Put("dead", 123)

	v, ok := c.Get("dead")
	require.True(t, ok)
	require.Equal(t, 123, v)

	clock = clock.Add(20 * time.Millisecond)

	v, ok = c.Get("dead")
	require.False(t, ok)
	require.Nil(t, v)

	_, ok = c.Get("dead")
	require.False(t, ok)

	snap := c.Snapshot()
	_, present := snap["dead"]
	require.False(t, present)
}