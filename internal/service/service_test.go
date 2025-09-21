package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	gorm "github.com/jinzhu/gorm"
	log "github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/require"

	"l0-demo/internal/models"
	"l0-demo/internal/repository"
	svc "l0-demo/internal/service"
)

type pgStub struct {
	created           models.Order
	getResp           models.Order
	getErr            error
	getAllResp        []models.Order
	getAllErr         error
	createErr         error
	createOrUpdateErr error
}

func (p *pgStub) Create(ord models.Order) error       { p.created = ord; return p.createErr }
func (p *pgStub) CreateOrUpdate(o models.Order) error { p.created = o; return p.createOrUpdateErr }
func (p *pgStub) Get(string) (models.Order, error)    { return p.getResp, p.getErr }
func (p *pgStub) GetAll() ([]models.Order, error)     { return p.getAllResp, p.getAllErr }

type cacheStub struct {
	m        map[string]models.Order
	putCount int
}

type fakeOrderRepo struct{ called bool }

func (f *fakeOrderRepo) Create(o models.Order) error {
	f.called = true
	return nil
}
func (f *fakeOrderRepo) CreateOrUpdate(o models.Order) error {
	f.called = true
	return nil
}
func (f *fakeOrderRepo) GetAllDbOrders() ([]models.Order, error)     { return []models.Order{}, nil }
func (f *fakeOrderRepo) GetDbOrder(uid string) (models.Order, error) { return models.Order{}, nil }
func (f *fakeOrderRepo) Get(uid string) (models.Order, error)        { return models.Order{}, nil }
func (f *fakeOrderRepo) GetAll() ([]models.Order, error)             { return []models.Order{}, nil }

type fakeCache struct{}

func (f *fakeCache) PutOrder(uid string, o models.Order)             {}
func (f *fakeCache) GetAllOrders() ([]models.Order, error)           { return []models.Order{}, nil }
func (f *fakeCache) GetAllCachedOrders() ([]models.Order, error)     { return []models.Order{}, nil }
func (f *fakeCache) GetCachedOrder(uid string) (models.Order, error) { return models.Order{}, nil }
func (f *fakeCache) GetOrder(uid string) (models.Order, error)       { return models.Order{}, nil }

var _ repository.OrderPostgres = (*fakeOrderRepo)(nil)
var _ repository.OrderCache = (*fakeCache)(nil)
var _ repository.OrderPostgres = (*pgStub)(nil)
var _ repository.OrderCache = (*cacheStub)(nil)

type countingCache struct {
	cacheStub
	puts int
}

func (c *countingCache) PutOrder(uid string, o models.Order) {
	c.cacheStub.PutOrder(uid, o)
	c.puts++
}

type pgWithData struct {
	pgStub
	orders []models.Order
}

func (p *pgWithData) GetAll() ([]models.Order, error) { return p.orders, nil }

func TestService_PutOrdersFromDbToCache_SkipsInvalid_LogsWarn(t *testing.T) {
	hook := logtest.NewGlobal()
	defer hook.Reset()

	bad := models.Order{OrderUid: "short"}
	good := makeValidOrder(strings.Repeat("g", 19))

	repo := &pgWithData{orders: []models.Order{bad, good}}
	cc := &countingCache{}
	s := svc.NewService(&repository.Repository{OrderPostgres: repo, OrderCache: cc})

	require.NoError(t, s.PutOrdersFromDbToCache())

	require.Equal(t, 1, cc.puts)

	entries := hook.AllEntries()
	require.NotEmpty(t, entries)
	found := false
	for _, e := range entries {
		if e.Level == log.WarnLevel && e.Message == "skip invalid order from DB" && e.Data["uid"] == bad.OrderUid {
			found = true
			break
		}
	}
	require.True(t, found, "expected warn log for invalid order")
}

func (c *cacheStub) PutOrder(id string, o models.Order) {
	if c.m == nil {
		c.m = map[string]models.Order{}
	}
	c.m[id] = o
	c.putCount++
}

func (c *cacheStub) GetOrder(uid string) (models.Order, error) { return c.m[uid], nil }
func (c *cacheStub) GetAllOrders() ([]models.Order, error) {
	var a []models.Order
	for _, v := range c.m {
		a = append(a, v)
	}
	return a, nil
}

type repoStub struct {
	repository.OrderPostgres
	repository.OrderCache
}

func makeValidOrder(uid string) models.Order {
	return models.Order{
		OrderUid:        uid,
		TrackNumber:     strings.Repeat("1", 14),
		Entry:           "ENTR",
		Locale:          "ru",
		CustomerId:      "CUST",
		DeliveryService: "SERVE",
		DateCreated:     time.Now(),
		OofShard:        "1",

		Delivery: &models.Delivery{
			Name:    "Alex",
			Phone:   "123456",
			Zip:     "12345",
			City:    "Moscow",
			Address: "Lenina 1",
			Region:  "RU",
			Email:   "a@b.co",
		},
		Payment: &models.Payment{
			Transaction:  "tx",
			Currency:     "RUB",
			Provider:     "prov",
			Amount:       1,
			PaymentDt:    1,
			Bank:         "bank",
			DeliveryCost: 1,
			GoodsTotal:   1,
			CustomFee:    0,
		},
		Items: []models.Item{{
			ChrtId:      1,
			TrackNumber: strings.Repeat("1", 14),
			Price:       1,
			Rid:         strings.Repeat("r", 21),
			Name:        "item",
			Sale:        1,
			Size:        "S",
			TotalPrice:  1,
			NmId:        1,
			Brand:       "brand",
			Status:      0,
		}},
	}
}

func TestService_PutDbOrder_DelegatesToRepo(t *testing.T) {
	fr := &fakeOrderRepo{}
	fc := &fakeCache{}
	deps := &repository.Repository{OrderPostgres: fr, OrderCache: fc}
	s := svc.NewService(deps)

	ord := makeValidOrder(strings.Repeat("a", 19))

	if err := s.PutDbOrder(ord); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if !fr.called {
		t.Fatal("expected repo.Create/CreateOrUpdate to be called")
	}
}

func TestService_GetDbOrder_OK(t *testing.T) {
	p := &pgStub{getResp: models.Order{OrderUid: strings.Repeat("u", 19)}}
	s := svc.NewService(&repository.Repository{OrderPostgres: p, OrderCache: &cacheStub{}})

	out, err := s.GetDbOrder(strings.Repeat("u", 19))
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("u", 19), out.OrderUid)
}

func TestService_GetDbOrder_NotFound_Maps(t *testing.T) {
	p := &pgStub{getErr: gorm.ErrRecordNotFound}
	s := svc.NewService(&repository.Repository{OrderPostgres: p, OrderCache: &cacheStub{}})

	_, err := s.GetDbOrder("nope")
	require.ErrorIs(t, err, svc.ErrNotFound)
}

func TestService_HandleMessage_Errors_And_FillsDate(t *testing.T) {

	s := svc.NewService(&repository.Repository{OrderPostgres: &pgStub{}, OrderCache: &cacheStub{}})
	require.Error(t, s.HandleMessage(context.Background(), []byte("not json")))

	c := &cacheStub{}
	p := &pgStub{}
	s = svc.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	msg := makeValidOrder(strings.Repeat("u", 19))
	msg.DateCreated = time.Time{}
	b, _ := json.Marshal(msg)

	require.NoError(t, s.HandleMessage(context.Background(), b))

	got := c.m[msg.OrderUid]
	require.False(t, got.DateCreated.IsZero(), "DateCreated should be set")
	require.Equal(t, msg.OrderUid, p.created.OrderUid)
}

func TestHandleMessage(t *testing.T) {
	ord := makeValidOrder(strings.Repeat("u", 19))
	b, _ := json.Marshal(ord)
	r := &repoStub{OrderPostgres: &pgStub{}, OrderCache: &cacheStub{}}
	s := svc.NewService(&repository.Repository{OrderPostgres: r.OrderPostgres, OrderCache: r.OrderCache})

	require.NoError(t, s.HandleMessage(context.Background(), b))
}

func TestService_CacheMethods(t *testing.T) {
	c := &cacheStub{}
	s := svc.NewService(&repository.Repository{OrderCache: c, OrderPostgres: &pgStub{}})

	order := models.Order{OrderUid: "u1"}
	s.PutCachedOrder(order)

	got, err := s.GetCachedOrder("u1")
	require.NoError(t, err)
	require.Equal(t, order, got)

	all, err := s.GetAllCachedOrders()
	require.NoError(t, err)
	require.Len(t, all, 1)
	require.Equal(t, order, all[0])
}

func TestService_DbMethods(t *testing.T) {
	p := &pgStub{created: models.Order{OrderUid: "u1"}}
	c := &cacheStub{}
	s := svc.NewService(&repository.Repository{OrderCache: c, OrderPostgres: p})

	orders, err := s.GetAllDbOrders()
	require.NoError(t, err)
	require.Len(t, orders, 0)

	err = s.PutOrdersFromDbToCache()
	require.NoError(t, err)
}

func TestService_PutOrdersFromDbToCache_PropagatesError(t *testing.T) {
	p := &pgStub{getAllErr: fmt.Errorf("db fail")}
	c := &cacheStub{}
	s := svc.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	err := s.PutOrdersFromDbToCache()
	require.Error(t, err)
	require.Contains(t, err.Error(), "db fail")
	require.Equal(t, 0, c.putCount)
}

func TestService_HandleMessage_CreateOrUpdate_Error(t *testing.T) {
	p := &pgStub{createOrUpdateErr: fmt.Errorf("write failed")}
	c := &cacheStub{}
	s := svc.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	msg := makeValidOrder(strings.Repeat("e", 19))
	b, _ := json.Marshal(msg)

	err := s.HandleMessage(context.Background(), b)
	require.Error(t, err)
	require.Contains(t, err.Error(), "write failed")

	_, ok := c.m[msg.OrderUid]
	require.False(t, ok, "order must not be cached on repo error")
}

func TestPutDbOrder_ValidationFails(t *testing.T) {
	r := &repository.Repository{
		OrderPostgres: &fakeOrderRepo{},
		OrderCache:    &fakeCache{},
	}
	s := svc.NewService(r)

	bad := models.Order{
		OrderUid: "short",
		Items:    []models.Item{},
		Delivery: nil,
		Payment:  nil,
	}

	if err := s.PutDbOrder(bad); err == nil {
		t.Fatal("expected validation error, got nil")
	}
}

func TestPutDbOrder_ValidCallsRepo(t *testing.T) {
	fr := &fakeOrderRepo{}
	r := &repository.Repository{
		OrderPostgres: fr,
		OrderCache:    &fakeCache{},
	}
	s := svc.NewService(r)

	good := models.Order{
		OrderUid:        strings.Repeat("a", 19),
		TrackNumber:     strings.Repeat("1", 14),
		Entry:           "ENTR",
		Locale:          "ru",
		CustomerId:      "CUST",
		DeliveryService: "SERVE",
		SmId:            10,
		DateCreated:     time.Now(),
		OofShard:        "1",
		Delivery: &models.Delivery{
			Name: "Alex", Phone: "123", Zip: "12345",
			City: "Moscow", Address: "Lenina 1", Region: "RU",
			Email: "a@b.co",
		},
		Payment: &models.Payment{
			Transaction: "tx", Currency: "RUB", Provider: "prov",
			Amount: 1, PaymentDt: 1, Bank: "bank",
			DeliveryCost: 1, GoodsTotal: 1, CustomFee: 0,
		},
		Items: []models.Item{{
			ChrtId: 1, TrackNumber: strings.Repeat("1", 14),
			Price: 1, Rid: strings.Repeat("r", 21), Name: "name",
			Sale: 1, Size: "S", TotalPrice: 1, NmId: 1, Brand: "brand",
			Status: 0,
		}},
	}

	if err := s.PutDbOrder(good); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if !fr.called {
		t.Fatal("expected repo.Create to be called for valid order")
	}
}
