package service_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	gorm "github.com/jinzhu/gorm"
	"github.com/stretchr/testify/require"

	"l0-demo/internal/models"
	"l0-demo/internal/repository"
	"l0-demo/internal/service"
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
	m map[string]models.Order 
	putCount int
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

type repoStub struct{ repository.OrderPostgres; repository.OrderCache }

func TestService_PutDbOrder_DelegatesToRepo(t *testing.T) {
	p := &pgStub{}
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: &cacheStub{}})

	in := models.Order{OrderUid: "u1"}
	require.NoError(t, s.PutDbOrder(in))
	require.Equal(t, in, p.created)
}


func TestService_GetDbOrder_OK(t *testing.T) {
	p := &pgStub{ getResp: models.Order{OrderUid: "u1"} }
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: &cacheStub{}})

	out, err := s.GetDbOrder("u1")
	require.NoError(t, err)
	require.Equal(t, "u1", out.OrderUid)
}

func TestService_GetDbOrder_NotFound_Maps(t *testing.T) {
	p := &pgStub{ getErr: gorm.ErrRecordNotFound }
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: &cacheStub{}})

	_, err := s.GetDbOrder("nope")
	require.ErrorIs(t, err, service.ErrNotFound)
}

func TestService_HandleMessage_Errors_And_FillsDate(t *testing.T) {

	s := service.NewService(&repository.Repository{OrderPostgres: &pgStub{}, OrderCache: &cacheStub{}})
	require.Error(t, s.HandleMessage(context.Background(), []byte("not json")))

	c := &cacheStub{}
	p := &pgStub{}
	s = service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	msg := models.Order{OrderUid: "u1"}
	b, _ := json.Marshal(msg)

	require.NoError(t, s.HandleMessage(context.Background(), b))

	got := c.m["u1"]
	require.False(t, got.DateCreated.IsZero(), "DateCreated should be set")

	require.Equal(t, "u1", p.created.OrderUid)
}

func TestHandleMessage(t *testing.T) {
	ord := models.Order{OrderUid:"u1", DateCreated: time.Now().UTC()}
	b, _ := json.Marshal(ord)
	r := &repoStub{OrderPostgres:&pgStub{}, OrderCache:&cacheStub{}}
	s := service.NewService(&repository.Repository{OrderPostgres: r.OrderPostgres, OrderCache: r.OrderCache})

	if err := s.HandleMessage(context.Background(), b); err != nil {
		t.Fatalf("HandleMessage error: %v", err)
	}
}

func TestService_CacheMethods(t *testing.T) {
	c := &cacheStub{}
	s := service.NewService(&repository.Repository{OrderCache: c, OrderPostgres: &pgStub{}})

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
	s := service.NewService(&repository.Repository{OrderCache: c, OrderPostgres: p})

	orders, err := s.GetAllDbOrders()
	require.NoError(t, err)
	require.Len(t, orders, 0)

	err = s.PutOrdersFromDbToCache()
	require.NoError(t, err)
}

func TestService_PutOrdersFromDbToCache_PropagatesError(t *testing.T) {
	p := &pgStub{ getAllErr: fmt.Errorf("db fail") }
	c := &cacheStub{}
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	err := s.PutOrdersFromDbToCache()
	require.Error(t, err)
	require.Contains(t, err.Error(), "db fail")
	require.Equal(t, 0, c.putCount)
}

func TestService_PutOrdersFromDbToCache_PutsAll(t *testing.T) {
	orders := []models.Order{
		{OrderUid: "u1"}, {OrderUid: "u2"}, {OrderUid: "u3"},
	}
	p := &pgStub{ getAllResp: orders }
	c := &cacheStub{}
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	require.NoError(t, s.PutOrdersFromDbToCache())
	require.Equal(t, len(orders), c.putCount)
	require.Len(t, c.m, len(orders))
	require.NotZero(t, c.m["u1"])
	require.NotZero(t, c.m["u2"])
	require.NotZero(t, c.m["u3"])
}

func TestService_HandleMessage_CreateOrUpdate_Error(t *testing.T) {
	p := &pgStub{ createOrUpdateErr: fmt.Errorf("write failed") }
	c := &cacheStub{}
	s := service.NewService(&repository.Repository{OrderPostgres: p, OrderCache: c})

	msg := models.Order{ OrderUid: "u_err" }
	b, _ := json.Marshal(msg)

	err := s.HandleMessage(context.Background(), b)
	require.Error(t, err)
	require.Contains(t, err.Error(), "write failed")

	_, ok := c.m["u_err"]
	require.False(t, ok, "order must not be cached on repo error")
}