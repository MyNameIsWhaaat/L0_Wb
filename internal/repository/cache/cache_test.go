package cache_test

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"l0-demo/internal/models"
	"l0-demo/internal/repository/cache"
)

func TestOrderCache_PutGet_All(t *testing.T) {
	cch := cache.NewOrderCache(cache.NewCache())

	_, err := cch.GetOrder("nope")
	require.Error(t, err)
	if eh, ok := err.(cache.ErrorHandler); ok {
		require.Equal(t, http.StatusNotFound, eh.StatusCode)
	}

	in := models.Order{OrderUid: "u1", CustomerId: "cust"}
	cch.PutOrder(in.OrderUid, in)

	got, err := cch.GetOrder("u1")
	require.NoError(t, err)
	require.Equal(t, "u1", got.OrderUid)

	all, err := cch.GetAllOrders()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, "u1", all[0].OrderUid)
}

func TestErrorHandler_ErrorString(t *testing.T) {
	e := cache.NewErrorHandler(fmt.Errorf("kaboom"), http.StatusInternalServerError)
	require.Equal(t, "kaboom", e.Error())
	require.Equal(t, http.StatusInternalServerError, e.StatusCode)
}

func TestOrderCache_GetOrder_ConvertError_500(t *testing.T) { 
	base := cache.NewCache()
	base.Put("bad", "not-an-order")

	cch := cache.NewOrderCache(base)

	_, err := cch.GetOrder("bad")
	require.Error(t, err)
	eh, ok := err.(cache.ErrorHandler)
	require.True(t, ok, "err should be cache.ErrorHandler")
	require.Equal(t, http.StatusInternalServerError, eh.StatusCode)
	require.Contains(t, eh.Error(), "failed to convert order") 
}

func TestOrderCache_GetAll_Empty_OK(t *testing.T) {
	cch := cache.NewOrderCache(cache.NewCache())

	out, err := cch.GetAllOrders()
	require.NoError(t, err)
	require.Len(t, out, 0)
}

func TestOrderCache_GetAll_ConvertError_500(t *testing.T) {
	base := cache.NewCache() 
	base.Put("u1", models.Order{OrderUid: "u1"}) 
	base.Put("oops", 12345)

	cch := cache.NewOrderCache(base)

	out, err := cch.GetAllOrders()
	require.Nil(t, out)
	eh, ok := err.(cache.ErrorHandler)
	require.True(t, ok)
	require.Equal(t, http.StatusInternalServerError, eh.StatusCode)
	require.Contains(t, eh.Error(), "failed to convert order")
}

func TestOrderCache_Delete_RemovesKey(t *testing.T) {
	base := cache.NewCache()
	cch := cache.NewOrderCache(base)

	o := models.Order{OrderUid: "to_del"}
	cch.PutOrder(o.OrderUid, o)
 
	cch.Delete("to_del")

	_, err := cch.GetOrder("to_del")
	require.Error(t, err)
	eh, ok := err.(cache.ErrorHandler)
	require.True(t, ok)
	require.Equal(t, http.StatusNotFound, eh.StatusCode)
}

func TestCache_WithTTL_JanitorAndClose(t *testing.T) {
    ttl := 30 * time.Millisecond
    c := cache.NewCache(cache.WithTTL(ttl))
    defer c.Close()

    c.Put("k", 1)
    _, ok := c.Get("k")
    require.True(t, ok)

    time.Sleep(ttl + ttl/2 + 20*time.Millisecond)

    snap := c.Snapshot()
    _, present := snap["k"]
    require.False(t, present)
}

func TestCache_WithNoJanitor_Close_NoTicker(t *testing.T) {
    c := cache.NewCache(cache.WithNoJanitor())
    defer c.Close()

    c.Put("a", 1)
    time.Sleep(20 * time.Millisecond)

    snap := c.Snapshot()
    _, present := snap["a"]
    require.True(t, present)
}

func TestCache_WithNoJanitor_ThenTTL_TickerStarts(t *testing.T) {
    ttl := 15 * time.Millisecond
    c := cache.NewCache(cache.WithNoJanitor(), cache.WithTTL(ttl))
    defer c.Close()

    c.Put("x", 42)
    time.Sleep(ttl + ttl/2 + 20*time.Millisecond)

    _, ok := c.Get("x")
    require.False(t, ok)
}

func TestCache_Get_WrappedValue_ReturnsUnderlying(t *testing.T) {
	c := cache.NewCache()
	t.Cleanup(c.Close)

	c.Put("k", "v")
	got, ok := c.Get("k")
	require.True(t, ok)
	require.Equal(t, "v", got)
}

func TestCache_Get_LegacyRawValue_ReturnsAsIs(t *testing.T) {
	c := cache.NewCache()
	t.Cleanup(c.Close)

	c.Mutex.Lock()
	c.Data["legacy"] = "raw"
	c.Mutex.Unlock()

	got, ok := c.Get("legacy")
	require.True(t, ok)
	require.Equal(t, "raw", got)
}

func TestCache_Snapshot_Mixed_WrappedAndRaw(t *testing.T) {
	c := cache.NewCache(cache.WithTTL(1 * time.Hour))
	t.Cleanup(c.Close)

	c.Put("wrapped", 123)

	c.Mutex.Lock()
	c.Data["raw"] = "plain"
	c.Mutex.Unlock()

	snap := c.Snapshot()
	require.Equal(t, 2, len(snap))
	require.Equal(t, 123, snap["wrapped"])
	require.Equal(t, "plain", snap["raw"])
}