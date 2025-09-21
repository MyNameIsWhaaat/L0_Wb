package http_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	httpdelivery "l0-demo/internal/delivery/http"
	"l0-demo/internal/models"
	"l0-demo/internal/repository/cache"
	"l0-demo/internal/service"
)

type svcStub struct {
	getCached        func(uid string) (models.Order, error)
	getAllCached     func() ([]models.Order, error)
	getAllDb         func() ([]models.Order, error)
	getDb            func(uid string) (models.Order, error)
	putFromDbToCache func() error
	putCached        func(order models.Order)
	putDb            func(order models.Order) error
	handle           func(ctx context.Context, payload []byte) error
}

var _ service.Order = (*svcStub)(nil)

func (s *svcStub) GetCachedOrder(uid string) (models.Order, error) {
	if s.getCached != nil {
		return s.getCached(uid)
	}
	return models.Order{}, fmt.Errorf("not implemented")
}
func (s *svcStub) GetAllCachedOrders() ([]models.Order, error) {
	if s.getAllCached != nil {
		return s.getAllCached()
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *svcStub) GetAllDbOrders() ([]models.Order, error) {
	if s.getAllDb != nil {
		return s.getAllDb()
	}
	return nil, fmt.Errorf("not implemented")
}
func (s *svcStub) GetDbOrder(uid string) (models.Order, error) {
	if s.getDb != nil {
		return s.getDb(uid)
	}
	return models.Order{}, service.ErrNotFound
}
func (s *svcStub) PutOrdersFromDbToCache() error {
	if s.putFromDbToCache != nil {
		return s.putFromDbToCache()
	}
	return fmt.Errorf("not implemented")
}
func (s *svcStub) PutCachedOrder(order models.Order) {
	if s.putCached != nil {
		s.putCached(order)
	}
}
func (s *svcStub) PutDbOrder(order models.Order) error {
	if s.putDb != nil {
		return s.putDb(order)
	}
	return fmt.Errorf("not implemented")
}
func (s *svcStub) HandleMessage(ctx context.Context, payload []byte) error {
	if s.handle != nil {
		return s.handle(ctx, payload)
	}
	return nil
}

func newRouter(s *svcStub) http.Handler {
	h := httpdelivery.NewHandler(s)
	return h.InitRoutes()
}

const sampleOrderJSON = `{
  "order_uid":"b563feb7b2b84b6test",
  "track_number":"WBILMTESTTRACK",
  "entry":"WBIL",
  "locale":"en",
  "internal_signature":"",
  "customer_id":"test",
  "delivery_service":"meest",
  "shardkey":"9",
  "sm_id":99,
  "date_created":"2021-11-26T06:22:19Z",
  "oof_shard":"1",
  "delivery":{"name":"Test Testov","phone":"+9720000000","zip":"2639809","city":"Kiryat Mozkin","address":"Ploshad Mira 15","region":"Kraiot","email":"test@gmail.com"},
  "payment":{"transaction":"b563feb7b2b84b6test","request_id":"","currency":"USD","provider":"wbpay","amount":1817,"payment_dt":1637907727,"bank":"alpha","delivery_cost":1500,"goods_total":317,"custom_fee":0},
  "items":[{"chrt_id":9934930,"track_number":"WBILMTESTTRACK","price":453,"rid":"ab4219087a764ae0btest","name":"Mascaras","sale":30,"size":"0","total_price":317,"nm_id":2389212,"brand":"Vivienne Sabo","status":202}]
}`

func mustOrder(t *testing.T) models.Order {
	t.Helper()
	var o models.Order
	require.NoError(t, json.Unmarshal([]byte(sampleOrderJSON), &o))
	if o.DateCreated.IsZero() {
		o.DateCreated = time.Date(2021, 11, 26, 6, 22, 19, 0, time.UTC)
	}
	return o
}

//
// ---------- infra / misc ----------
//

func Test_Server_Run_Shutdown(t *testing.T) {
	s := &httpdelivery.Server{}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	go func() {
		err := s.Run(":0", handler)
		if err != nil && err != http.ErrServerClosed {
			t.Error(err)
		}
	}()

	time.Sleep(50 * time.Millisecond)
	require.NoError(t, s.Shutdown(context.Background()))
}

func TestHandler_NoRoute(t *testing.T) {
	r := newRouter(&svcStub{})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code)
}

//
// ---------- /api/orders ----------
//

func Test_GetAllOrders_OK(t *testing.T) {
	o := mustOrder(t)
	r := newRouter(&svcStub{
		getAllCached: func() ([]models.Order, error) { return []models.Order{o}, nil },
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"data":[`)
	require.Contains(t, w.Body.String(), `"order_uid":"b563feb7b2b84b6test"`)
}

func Test_GetAllOrders_InternalError_500(t *testing.T) {
	r := newRouter(&svcStub{
		getAllCached: func() ([]models.Order, error) { return nil, fmt.Errorf("cache down") },
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "cache down")
}

func Test_GetAllOrders_CacheCustomStatus(t *testing.T) {
	r := newRouter(&svcStub{
		getAllCached: func() ([]models.Order, error) {
			return nil, cache.NewErrorHandler(fmt.Errorf("cache unavailable"), http.StatusServiceUnavailable)
		},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusServiceUnavailable, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "cache unavailable")
}

//
// ---------- /api/order/:uid (cache) ----------
//

func Test_GetOrderById_CacheHit_OK(t *testing.T) {
	o := mustOrder(t)
	r := newRouter(&svcStub{
		getCached: func(uid string) (models.Order, error) { return o, nil },
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/"+o.OrderUid, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"order_uid":"`+o.OrderUid+`"`)
}

func Test_GetOrderById_CacheMiss_404(t *testing.T) {
	r := newRouter(&svcStub{
		getCached: func(string) (models.Order, error) { return models.Order{}, service.ErrNotFound },
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/does_not_exist", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "not found")
}

func Test_GetOrderById_CacheCustomStatus(t *testing.T) {
	r := newRouter(&svcStub{
		getCached: func(string) (models.Order, error) {
			return models.Order{}, cache.NewErrorHandler(fmt.Errorf("rate limited"), 429)
		},
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/any", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, 429, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "rate limited")
}

func Test_GetOrderById_InternalError_500(t *testing.T) {
	r := newRouter(&svcStub{
		getCached: func(string) (models.Order, error) { return models.Order{}, fmt.Errorf("boom") },
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/uid", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "boom")
}

func Test_GetOrderById_InvalidUID_400(t *testing.T) {
	r := newRouter(&svcStub{})
	w := httptest.NewRecorder()
	// "%20%20" -> "  " после URL decode; требует TrimSpace в хендлере
	req := httptest.NewRequest(http.MethodGet, "/api/order/%20%20", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "invalid uid")
}

//
// ---------- /api/order/db/:uid (DB) ----------
//

func Test_GetDbOrderById_DbHit_OK(t *testing.T) {
	o := mustOrder(t)
	r := newRouter(&svcStub{
		getDb: func(uid string) (models.Order, error) {
			require.Equal(t, o.OrderUid, uid)
			return o, nil
		},
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/db/"+o.OrderUid, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"order_uid":"`+o.OrderUid+`"`)
}

func Test_GetDbOrderById_NotFound_404(t *testing.T) {
	r := newRouter(&svcStub{
		getDb: func(string) (models.Order, error) { return models.Order{}, service.ErrNotFound },
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/db/no_db", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "not found")
}

func Test_GetDbOrderById_InternalError_500(t *testing.T) {
	r := newRouter(&svcStub{
		getDb: func(string) (models.Order, error) { return models.Order{}, fmt.Errorf("db exploded") },
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/db/whatever", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "db exploded")
}

func Test_GetDbOrderById_MissingUID_400(t *testing.T) {
	r := newRouter(&svcStub{})
	w := httptest.NewRecorder()
	// Требует TrimSpace в GetDbOrderById
	req := httptest.NewRequest(http.MethodGet, "/api/order/db/%20%20", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusBadRequest, w.Code, "body=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "missing uid")
}