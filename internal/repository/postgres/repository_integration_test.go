package postgres_test

import (
	"testing"
	"time"

	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"l0-demo/internal/models"
	pg "l0-demo/internal/repository/postgres"
	"l0-demo/internal/repository"
)

func Test_Postgres_FullCoverage(t *testing.T) {
	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	resource, err := pool.Run("postgres", "16-alpine", []string{
		"POSTGRES_DB=orders",
		"POSTGRES_USER=app",
		"POSTGRES_PASSWORD=app",
	})
	require.NoError(t, err)

	t.Cleanup(func() { _ = pool.Purge(resource) })

	var dbWrapper *repository.Repository

	require.NoError(t, pool.Retry(func() error {
		hostPort := resource.GetPort("5432/tcp")
		db, err := pg.ConnectDB(pg.Config{
			Host:     "localhost",
			Port:     hostPort,
			Username: "app",
			Password: "app",
			DbName:   "orders",
			SslMode:  "disable",
		})
		if err != nil {
			return err
		}

		require.NoError(t, db.AutoMigrate(&models.Order{}, &models.Delivery{}, &models.Payment{}, &models.Item{}).Error)

		dbWrapper = repository.NewRepository(db)
		return nil
	}))

	orderWithItems := models.Order{
		OrderUid:        "uid_1",
		TrackNumber:     "TRACK1",
		Entry:           "ENT1",
		Locale:          "en",
		CustomerId:      "cust1",
		DeliveryService: "meest",
		ShardKey:        "1",
		SmId:            10,
		DateCreated:     time.Now(),
		OofShard:        "1",
		Delivery: models.Delivery{
			Name:    "Test User",
			Phone:   "+123456",
			Zip:     "11111",
			City:    "City",
			Address: "Addr",
			Region:  "Region",
			Email:   "test@mail.com",
		},
		Payment: models.Payment{
			Transaction:  "uid_1",
			Currency:     "USD",
			Provider:     "pay",
			Amount:       1000,
			PaymentDt:    123456,
			Bank:         "bank",
			DeliveryCost: 100,
			GoodsTotal:   900,
			CustomFee:    0,
		},
		Items: []models.Item{{
			ChrtId:      1,
			TrackNumber: "TRACK1",
			Price:       900,
			Rid:         "rid1",
			Name:        "Item1",
			Sale:        0,
			Size:        "M",
			TotalPrice:  900,
			NmId:        1,
			Brand:       "Brand1",
			Status:      200,
		}},
	}

	err = dbWrapper.OrderPostgres.CreateOrUpdate(orderWithItems)
	require.NoError(t, err)

	got, err := dbWrapper.OrderPostgres.Get("uid_1")
	require.NoError(t, err)
	require.Equal(t, "uid_1", got.OrderUid)
	require.Equal(t, 1, len(got.Items))
	require.Equal(t, "Item1", got.Items[0].Name)

	orderWithItems.Items = []models.Item{}
	orderWithItems.Delivery.Name = "Updated User"
	orderWithItems.Payment.Amount = 2000

	err = dbWrapper.OrderPostgres.CreateOrUpdate(orderWithItems)
	require.NoError(t, err)

	got2, err := dbWrapper.OrderPostgres.Get("uid_1")
	require.NoError(t, err)
	require.Equal(t, "Updated User", got2.Delivery.Name)
	require.Equal(t, 2000, got2.Payment.Amount)
	require.Equal(t, 0, len(got2.Items))

	orderNoItems := models.Order{
		OrderUid:        "uid_2",
		TrackNumber:     "TRACK2",
		Entry:           "ENT2",
		Locale:          "en",
		CustomerId:      "cust2",
		DeliveryService: "meest",
		ShardKey:        "2",
		SmId:            20,
		DateCreated:     time.Now(),
		OofShard:        "2",
		Delivery: models.Delivery{
			Name: "User2",
		},
		Payment: models.Payment{
			Transaction: "uid_2",
			Amount:      500,
		},
		Items: []models.Item{},
	}

	err = dbWrapper.OrderPostgres.CreateOrUpdate(orderNoItems)
	require.NoError(t, err)

	allOrders, err := dbWrapper.OrderPostgres.GetAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(allOrders), 2)

	db, _ := pg.ConnectDB(pg.Config{
		Host:     "localhost",
		Port:     resource.GetPort("5432/tcp"),
		Username: "app",
		Password: "app",
		DbName:   "orders",
		SslMode:  "disable",
	})
	defer db.Close()

	if err := db.Where("order_uid = ?", "uid_1").Delete(&models.Order{}).Error; err != nil {
		require.NoError(t, err)
	}

	_, err = dbWrapper.OrderPostgres.Get("uid_1")
	require.Error(t, err)

}
