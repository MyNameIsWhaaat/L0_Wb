package service_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"l0-demo/internal/models"
	"l0-demo/internal/repository"
	"l0-demo/internal/service"

	"github.com/stretchr/testify/require"
)

type pgStub struct{ created models.Order }
func (p *pgStub) Create(ord models.Order) error {p.created = ord; return nil}
func (p *pgStub) CreateOrUpdate(o models.Order) error { p.created = o; return nil }
func (p *pgStub) Get(string) (models.Order, error)    { return models.Order{}, nil }
func (p *pgStub) GetAll() ([]models.Order, error)     { return nil, nil }

type cacheStub struct{ m map[string]models.Order }
func (c *cacheStub) PutOrder(id string, o models.Order){ if c.m==nil {c.m=map[string]models.Order{}}; c.m[id]=o }
func (c *cacheStub) GetOrder(uid string)(models.Order,error){ return c.m[uid], nil }
func (c *cacheStub) GetAllOrders()([]models.Order,error){ var a []models.Order; for _,v:=range c.m{a=append(a,v)}; return a,nil }

type repoStub struct{ repository.OrderPostgres; repository.OrderCache }

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

