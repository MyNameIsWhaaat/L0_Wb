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
	"l0-demo/internal/service"
)

func Test_GetAllOrders_RegularError_500(t *testing.T) {
	s := &svcStub{
		getAllCached: func() ([]models.Order, error) {
			return nil, fmt.Errorf("regular error")
		},
	}
	h := httpdelivery.NewHandler(s)
	r := h.InitRoutes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Contains(t, w.Body.String(), "regular error")
}

type svcStub struct {
	getCached    func(uid string) (models.Order, error)
	getAllCached func() ([]models.Order, error)
	getAllDb     func() ([]models.Order, error)
	getDb        func(uid string) (models.Order, error)

	putFromDbToCache func() error
	putCached        func(order models.Order)
	putDb            func(order models.Order) error

	handle func(ctx context.Context, payload []byte) error
}

var _ service.Order = (*svcStub)(nil) 

func TestServer_Run_Shutdown(t *testing.T) {
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
	h := httpdelivery.NewHandler(&svcStub{})
	r := h.InitRoutes()
 
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/unknown", nil)
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusNotFound, w.Code) 
}

func (s *svcStub) GetCachedOrder(uid string) (models.Order, error) {
	if s.getCached != nil { return s.getCached(uid) }
	return models.Order{}, fmt.Errorf("not implemented")
}
func (s *svcStub) GetAllCachedOrders() ([]models.Order, error) {
	if s.getAllCached != nil { return s.getAllCached() }
	return nil, nil
}
func (s *svcStub) GetAllDbOrders() ([]models.Order, error) {
	if s.getAllDb != nil { return s.getAllDb() }
	return nil, nil
}
func (s *svcStub) GetDbOrder(uid string) (models.Order, error) {
	if s.getDb != nil { return s.getDb(uid) }
	return models.Order{}, service.ErrNotFound  
}
func (s *svcStub) PutOrdersFromDbToCache() error {
	if s.putFromDbToCache != nil { return s.putFromDbToCache() }
	return nil
}
func (s *svcStub) PutCachedOrder(order models.Order) {
	if s.putCached != nil { s.putCached(order) }
}
func (s *svcStub) PutDbOrder(order models.Order) error {
	if s.putDb != nil { return s.putDb(order) }
	return nil
}
func (s *svcStub) HandleMessage(ctx context.Context, payload []byte) error {
	if s.handle != nil { return s.handle(ctx, payload) }
	return nil
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
	var o models.Order
	require.NoError(t, json.Unmarshal([]byte(sampleOrderJSON), &o))
	if o.DateCreated.IsZero() {
		o.DateCreated = time.Date(2021, 11, 26, 6, 22, 19, 0, time.UTC)
	}
	return o
}

func Test_GetAllOrders_OK(t *testing.T) {
	o := mustOrder(t)
	s := &svcStub{
		getAllCached: func() ([]models.Order, error) { return []models.Order{o}, nil },
	}
	h := httpdelivery.NewHandler(s)
	r := h.InitRoutes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"data":[`)
	require.Contains(t, w.Body.String(), `"order_uid":"b563feb7b2b84b6test"`)
}

func Test_GetOrderById_CacheHit_OK(t *testing.T) {
	o := mustOrder(t)
	s := &svcStub{
		getCached: func(uid string) (models.Order, error) { return o, nil },
	}
	h := httpdelivery.NewHandler(s)
	r := h.InitRoutes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/"+o.OrderUid, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	require.Contains(t, w.Body.String(), `"order_uid":"`+o.OrderUid+`"`)
}

func Test_GetDbOrderById_NotFound_404(t *testing.T) {
	uid := "does_not_exist"
	s := &svcStub{
		getDb: func(string) (models.Order, error) {
			return models.Order{}, service.ErrNotFound
		},
	}
	h := httpdelivery.NewHandler(s)
	r := h.InitRoutes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/order/db/"+uid, nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "not found")
}
