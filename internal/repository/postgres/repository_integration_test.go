package postgres_test

import (
	"testing"
	"time"

	gorm "github.com/jinzhu/gorm"
	"github.com/ory/dockertest/v3"
	"github.com/stretchr/testify/require"

	"l0-demo/internal/models"
	repo "l0-demo/internal/repository"
	pg "l0-demo/internal/repository/postgres"
)

type pgEnv struct {
	pool     *dockertest.Pool
	resource *dockertest.Resource
	DB       *gorm.DB
	R        *repo.Repository
}

func upPostgres(t *testing.T) *pgEnv {
	t.Helper()

	pool, err := dockertest.NewPool("")
	require.NoError(t, err)

	resource, err := pool.Run("postgres", "16-alpine", []string{
		"POSTGRES_DB=orders",
		"POSTGRES_USER=app",
		"POSTGRES_PASSWORD=app",
	})
	require.NoError(t, err)

	env := &pgEnv{pool: pool, resource: resource}
	t.Cleanup(func() { _ = pool.Purge(resource) })

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
		env.DB = db

		if err := db.AutoMigrate(&models.Order{}, &models.Delivery{}, &models.Payment{}, &models.Item{}).Error; err != nil {
			return err
		}

		env.R = repo.NewRepository(db)
		return nil
	}))

	return env
}

func order(uid, track string, withItems bool) models.Order {
	o := models.Order{
		OrderUid:        uid,
		TrackNumber:     track,
		Entry:           "ENT",
		Locale:          "en",
		CustomerId:      "cust",
		DeliveryService: "meest",
		ShardKey:        "1",
		SmId:            10,
		DateCreated:     time.Now().UTC(),
		OofShard:        "1",
		Delivery: models.Delivery{
			Name: "User",
		},
		Payment: models.Payment{
			Transaction:  uid,
			Currency:     "USD",
			Provider:     "pay",
			Amount:       100,
			PaymentDt:    123,
			Bank:         "bank",
			DeliveryCost: 10,
			GoodsTotal:   90,
		},
	}
	if withItems {
		o.Items = []models.Item{{
			ChrtId:      1,
			TrackNumber: track,
			Price:       90,
			Rid:         "rid1",
			Name:        "Item1",
			Sale:        0,
			Size:        "M",
			TotalPrice:  90,
			NmId:        1,
			Brand:       "Brand1",
			Status:      200,
		}}
	} else {
		o.Items = []models.Item{}
	}
	return o
}

func Test_Postgres_CreateUpdateGet_GetAll_Positive(t *testing.T) {
	env := upPostgres(t)

	o1 := order("uid_1", "TRACK1", true)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o1))

	got, err := env.R.OrderPostgres.Get("uid_1")
	require.NoError(t, err)
	require.Equal(t, "uid_1", got.OrderUid)
	require.Len(t, got.Items, 1)
	require.Equal(t, "Item1", got.Items[0].Name)

	o1.Items = []models.Item{}
	o1.Delivery.Name = "Updated User"
	o1.Payment.Amount = 2000
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o1))

	got2, err := env.R.OrderPostgres.Get("uid_1")
	require.NoError(t, err)
	require.Equal(t, "Updated User", got2.Delivery.Name)
	require.Equal(t, 2000, got2.Payment.Amount)
	require.Len(t, got2.Items, 0)

	o2 := order("uid_2", "TRACK2", false)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o2))

	all, err := env.R.OrderPostgres.GetAll()
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(all), 2)

	require.NoError(t, env.DB.Where("order_uid = ?", "uid_1").Delete(&models.Order{}).Error)
	_, err = env.R.OrderPostgres.Get("uid_1")
	require.Error(t, err)
}

func Test_Postgres_Create_DuplicateUID_Error(t *testing.T) {
	env := upPostgres(t)

	o := order("dup_uid", "TRACKDUP", true)

	require.NoError(t, env.R.OrderPostgres.Create(o))

	err := env.R.OrderPostgres.Create(o)
	require.Error(t, err, "expected duplicate key error from Create")
}

func Test_Postgres_CreateOrUpdate_ItemsInsertError(t *testing.T) {
	env := upPostgres(t)

	if err := env.DB.DropTable(&models.Item{}).Error; err != nil {
		t.Fatalf("failed to drop items table: %v", err)
	}

	o := order("uid_items_err", "TRACK_ERR", true)

	err := env.R.OrderPostgres.CreateOrUpdate(o)
	require.Error(t, err, "expected error because items table is missing")
}

func Test_Postgres_GetAll_Empty_OK(t *testing.T) {
	env := upPostgres(t)

	all, err := env.R.OrderPostgres.GetAll()
	require.NoError(t, err)
	require.Len(t, all, 0)
}

func Test_CreateOrUpdate_CreateBranch_CreateError(t *testing.T) {
	env := upPostgres(t)

	require.NoError(t, env.DB.DropTable(&models.Order{}).Error)

	o := order("uid_new", "TRACK_NEW", true)
	err := env.R.OrderPostgres.CreateOrUpdate(o)
	require.Error(t, err, "expected error from tx.Create(&ord) on missing table")
}

func Test_CreateOrUpdate_UpdateBranch_OrderUpdatesError_UniqueTrack(t *testing.T) {
	env := upPostgres(t)

	require.NoError(t, env.DB.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS uq_orders_track ON orders (track_number)`).Error)

	a := order("uid_A", "TRACK_A", false)
	b := order("uid_B", "TRACK_B", false)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(a))
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(b))

	a.TrackNumber = "TRACK_B"
	err := env.R.OrderPostgres.CreateOrUpdate(a)
	require.Error(t, err, "expected unique constraint violation on orders(track_number)")
}

func Test_CreateOrUpdate_UpdateBranch_DeliveryUpdatesError_DroppedTable(t *testing.T) {
	env := upPostgres(t)

	o := order("uid_del_upd", "TRACK_DEL", false)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o))

	require.NoError(t, env.DB.DropTable(&models.Delivery{}).Error)

	o.Delivery.Name = "New Name"
	err := env.R.OrderPostgres.CreateOrUpdate(o)
	require.Error(t, err, "expected error updating deliveries after table drop")
}

func Test_CreateOrUpdate_UpdateBranch_PaymentUpdatesError_DroppedTable(t *testing.T) {
	env := upPostgres(t)

	o := order("uid_pay_upd", "TRACK_PAY", false)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o))

	require.NoError(t, env.DB.DropTable(&models.Payment{}).Error)

	o.Payment.Amount = 777
	err := env.R.OrderPostgres.CreateOrUpdate(o)
	require.Error(t, err, "expected error updating payments after table drop")
}

func Test_CreateOrUpdate_UpdateBranch_DeleteItemsError_DroppedTable(t *testing.T) {
	env := upPostgres(t)

	o := order("uid_del_items", "TRACK_DI", true)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o))

	require.NoError(t, env.DB.DropTable(&models.Item{}).Error)

	o.Items = []models.Item{{ChrtId: 2, TrackNumber: "TRACK_DI", Name: "X", Price: 1, TotalPrice: 1, NmId: 2, Status: 200}}
	err := env.R.OrderPostgres.CreateOrUpdate(o)
	require.Error(t, err, "expected error on DELETE from items after drop")
}

func Test_CreateOrUpdate_UpdateBranch_CreateItems_Success(t *testing.T) {
	env := upPostgres(t)

	o := order("uid_items_ok", "TRACK_OK", false)
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o))

	o.Items = []models.Item{{
		ChrtId:      123456,
		TrackNumber: o.TrackNumber,
		Price:       10,
		Name:        "NewItem",
		TotalPrice:  10,
		NmId:        1,
		Status:      200,
	}}
	require.NoError(t, env.R.OrderPostgres.CreateOrUpdate(o))

	got, err := env.R.OrderPostgres.Get("uid_items_ok")
	require.NoError(t, err)
	require.Len(t, got.Items, 1)
	require.Equal(t, 123456, got.Items[0].ChrtId)
	require.Equal(t, "NewItem", got.Items[0].Name)
}
