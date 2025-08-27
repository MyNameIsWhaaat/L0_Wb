package cache_test

import (
	"net/http"
	"testing"

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
