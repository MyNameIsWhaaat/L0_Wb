package cache_test

import (
	"fmt"
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

func TestErrorHandler_ErrorString(t *testing.T) {
	e := cache.NewErrorHandler(fmt.Errorf("kaboom"), http.StatusInternalServerError) 
	require.Equal(t, "kaboom", e.Error())
	require.Equal(t, http.StatusInternalServerError, e.StatusCode)
}

func TestOrderCache_GetOrder_ConvertError_500(t *testing.T) {
 
	base := cache.NewCache()
 
	base.Data["bad"] = "not-an-order" 
 
	cch := cache.NewOrderCache(base)

	_, err := cch.GetOrder("bad")
	require.Error(t, err)
	eh, ok := err.(cache.ErrorHandler)
	require.True(t, ok, "err should be cache.ErrorHandler")
	require.Equal(t, http.StatusInternalServerError, eh.StatusCode)
	require.Contains(t, eh.Error(), "failed to convert order with uid")
}

func TestOrderCache_GetAll_Empty_OK(t *testing.T) {
	cch := cache.NewOrderCache(cache.NewCache())

	out, err := cch.GetAllOrders()
	require.NoError(t, err)
	require.Len(t, out, 0) 
}

func TestOrderCache_GetAll_ConvertError_500(t *testing.T) {
	base := cache.NewCache()
 
	base.Data["oops"] = "not-an-order"

	cch := cache.NewOrderCache(base)

	out, err := cch.GetAllOrders()
	require.Nil(t, out)
	eh, ok := err.(cache.ErrorHandler)
	require.True(t, ok, "err should be cache.ErrorHandler")
	require.Equal(t, http.StatusInternalServerError, eh.StatusCode)
	require.Contains(t, eh.Error(), "failed to convert order with uid") 
}
