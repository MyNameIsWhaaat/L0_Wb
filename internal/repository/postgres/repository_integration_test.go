package postgres_test

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"

	"l0-demo/internal/models"
	pgrepo "l0-demo/internal/repository/postgres"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/ory/dockertest/v3"
	"github.com/ory/dockertest/v3/docker"
)

const (
	dbUser = "postgres"
	dbPass = "secret"
	dbName = "testdb"
)

var (
	db   *gorm.DB
	repo *pgrepo.OrderPostgresRepo
)

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func addCheckNotValid(t *testing.T, table, cname, condition string) {
	t.Helper()
	q := fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s) NOT VALID;`, table, cname, condition)
	if err := db.Exec(q).Error; err != nil {
		t.Fatalf("addCheckNotValid failed: %v\nquery: %s", err, q)
	}
}

func TestMain(m *testing.M) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("could not connect to docker: %v", err)
	}

	resource, err := pool.RunWithOptions(&dockertest.RunOptions{
		Repository: "postgres",
		Tag:        "13-alpine",
		Env: []string{
			"POSTGRES_PASSWORD=" + dbPass,
			"POSTGRES_USER=" + dbUser,
			"POSTGRES_DB=" + dbName,
		},
	}, func(hc *docker.HostConfig) {

		hc.AutoRemove = true
		hc.RestartPolicy = docker.RestartPolicy{Name: "no"}
	})
	if err != nil {
		log.Fatalf("could not start resource: %v", err)
	}

	defer func() {
		_ = pool.Purge(resource)
	}()

	hostPort := resource.GetPort("5432/tcp")

	var g *gorm.DB
	if err := pool.Retry(func() error {
		var e error
		connStr := fmt.Sprintf("host=localhost port=%s user=%s dbname=%s password=%s sslmode=disable",
			hostPort, dbUser, dbName, dbPass)
		g, e = gorm.Open("postgres", connStr)
		if e != nil {
			return e
		}
		return g.DB().Ping()
	}); err != nil {
		log.Fatalf("could not connect to postgres: %v", err)
	}

	g.DB().SetMaxOpenConns(10)
	g.DB().SetMaxIdleConns(5)
	g.DB().SetConnMaxLifetime(time.Minute)

	if err := g.AutoMigrate(
		&models.Order{},
		&models.Delivery{},
		&models.Payment{},
		&models.Item{},
	).Error; err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}

	db = g
	repo = pgrepo.NewOrderPostgres(db)

	code := m.Run()

	_ = db.Close()
	os.Exit(code)
}

func TestCreateAndGet(t *testing.T) {
	uid := "order-create-001"
	in := makeOrderFull(uid, 2)

	if err := repo.Create(in); err != nil {
		t.Fatalf("Create() error: %v", err)
	}

	got, err := repo.Get(uid)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}

	assertOrderHeaderEq(t, in, got)
	if got.Delivery == nil || got.Payment == nil {
		t.Fatalf("expected non-nil Delivery and Payment, got: %#v", got)
	}
	if len(got.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(got.Items))
	}
}

func TestCreateOrUpdate_InsertThenUpdate(t *testing.T) {
	uid := "order-upsert-001"

	initial := makeOrderFull(uid, 3)
	if err := repo.CreateOrUpdate(initial); err != nil {
		t.Fatalf("CreateOrUpdate(insert) error: %v", err)
	}

	got1, err := repo.Get(uid)
	if err != nil {
		t.Fatalf("Get(after insert) error: %v", err)
	}
	assertOrderHeaderEq(t, initial, got1)
	if len(got1.Items) != 3 {
		t.Fatalf("expected 3 items after insert, got %d", len(got1.Items))
	}

	updated := initial
	updated.TrackNumber = "TRACK-UPDATED"
	if updated.Delivery != nil {
		updated.Delivery.City = "Amsterdam"
	}
	if updated.Payment != nil {
		updated.Payment.Amount = updated.Payment.Amount + 777
	}
	updated.Items = []models.Item{
		makeItem(uid, "SKU-NEW-1", 111),
		makeItem(uid, "SKU-NEW-2", 222),
	}

	if err := repo.CreateOrUpdate(updated); err != nil {
		t.Fatalf("CreateOrUpdate(update) error: %v", err)
	}

	got2, err := repo.Get(uid)
	if err != nil {
		t.Fatalf("Get(after update) error: %v", err)
	}

	if got2.TrackNumber != "TRACK-UPDATED" {
		t.Fatalf("expected updated track number, got %s", got2.TrackNumber)
	}
	if got2.Delivery == nil || got2.Delivery.City != "Amsterdam" {
		t.Fatalf("delivery not updated: %#v", got2.Delivery)
	}
	if got2.Payment == nil || got2.Payment.Amount != updated.Payment.Amount {
		t.Fatalf("payment not updated: %#v", got2.Payment)
	}
	if len(got2.Items) != 2 {
		t.Fatalf("expected 2 items after replacement, got %d", len(got2.Items))
	}
	expectRIDs := map[string]bool{"SKU-NEW-1": true, "SKU-NEW-2": true}
	for _, it := range got2.Items {
		if !expectRIDs[it.Rid] {
			t.Fatalf("unexpected item after replacement (Rid check failed): %#v", it)
		}
	}
}

func TestCreateOrUpdate_WithNilChildren(t *testing.T) {
	uid := "order-nil-001"

	o := makeOrderHeaderOnly(uid)
	if err := repo.CreateOrUpdate(o); err != nil {
		t.Fatalf("CreateOrUpdate(header-only) error: %v", err)
	}

	got, err := repo.Get(uid)
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if got.Delivery != nil || got.Payment != nil || len(got.Items) != 0 {
		t.Fatalf("expected no children, got: delivery=%#v payment=%#v items=%d",
			got.Delivery, got.Payment, len(got.Items))
	}

	o2 := makeOrderFull(uid, 1)
	if err := repo.CreateOrUpdate(o2); err != nil {
		t.Fatalf("CreateOrUpdate(add-children) error: %v", err)
	}

	got2, err := repo.Get(uid)
	if err != nil {
		t.Fatalf("Get(after add children) error: %v", err)
	}
	if got2.Delivery == nil || got2.Payment == nil || len(got2.Items) != 1 {
		t.Fatalf("expected children added, got: delivery=%#v payment=%#v items=%d",
			got2.Delivery, got2.Payment, len(got2.Items))
	}
}

func TestGetAll(t *testing.T) {
	for i := 1; i <= 3; i++ {
		uid := fmt.Sprintf("order-all-%03d", i)
		if err := repo.Create(makeOrderFull(uid, i)); err != nil {
			t.Fatalf("Create(%s) error: %v", uid, err)
		}
	}

	all, err := repo.GetAll()
	if err != nil {
		t.Fatalf("GetAll() error: %v", err)
	}
	count := 0
	for _, o := range all {
		if len(o.OrderUid) > 0 && (len(o.Items) > 0 || o.Delivery != nil || o.Payment != nil) {
			count++
		}
	}
	if count < 3 {
		t.Fatalf("expected at least 3 orders, got %d/%d", count, len(all))
	}
}

func assertOrderHeaderEq(t *testing.T, want, got models.Order) {
	t.Helper()
	type header = struct {
		OrderUid          string
		TrackNumber       string
		Entry             string
		Locale            string
		InternalSignature string
		CustomerId        string
		DeliveryService   string
		ShardKey          string
		SmId              int
		OofShard          string
	}
	w := header{
		OrderUid:          want.OrderUid,
		TrackNumber:       want.TrackNumber,
		Entry:             want.Entry,
		Locale:            want.Locale,
		InternalSignature: want.InternalSignature,
		CustomerId:        want.CustomerId,
		DeliveryService:   want.DeliveryService,
		ShardKey:          want.ShardKey,
		SmId:              want.SmId,
		OofShard:          want.OofShard,
	}
	g := header{
		OrderUid:          got.OrderUid,
		TrackNumber:       got.TrackNumber,
		Entry:             got.Entry,
		Locale:            got.Locale,
		InternalSignature: got.InternalSignature,
		CustomerId:        got.CustomerId,
		DeliveryService:   got.DeliveryService,
		ShardKey:          got.ShardKey,
		SmId:              got.SmId,
		OofShard:          got.OofShard,
	}
	if w != g {
		t.Fatalf("order header mismatch:\nwant: %#v\ngot:  %#v", w, g)
	}
}

func makeOrderHeaderOnly(uid string) models.Order {
	return models.Order{
		OrderUid:          uid,
		TrackNumber:       "TRACK-" + uid,
		Entry:             "web",
		Locale:            "ru",
		InternalSignature: "",
		CustomerId:        "customer-1",
		DeliveryService:   "meest",
		ShardKey:          "9",
		SmId:              123,
		DateCreated:       time.Now().UTC(),
		OofShard:          "1",
	}
}

func makeOrderFull(uid string, items int) models.Order {
	o := makeOrderHeaderOnly(uid)

	o.Delivery = &models.Delivery{
		OrderRefer: uid,
		Name:       "John Doe",
		Phone:      "+100000000",
		Zip:        "000000",
		City:       "Moscow",
		Address:    "Some street, 1",
		Region:     "RU",
		Email:      "john@example.com",
	}
	o.Payment = &models.Payment{
		OrderRefer:   uid,
		Transaction:  "txn-" + trunc(uid, 15),
		Currency:     "RUB",
		Provider:     "cash",
		Amount:       1000,
		PaymentDt:    int(time.Now().Unix()),
		Bank:         "SBER",
		DeliveryCost: 300,
		GoodsTotal:   700,
		CustomFee:    0,
	}
	for i := 1; i <= items; i++ {
		o.Items = append(o.Items, makeItem(uid, fmt.Sprintf("SKU-%02d", i), 100*i))
	}

	return o
}

func makeItem(orderUID, sku string, price int) models.Item {
	return models.Item{
		OrderRefer:  orderUID,
		ChrtId:      1000 + price,
		TrackNumber: "TN-" + trunc(orderUID, 16),
		Price:       price,
		Rid:         sku,
		Name:        "Item " + sku,
		Sale:        0,
		Size:        "0",
		TotalPrice:  price,
		NmId:        1,
		Brand:       "WB",
		Status:      202,
	}
}

func execSQL(t *testing.T, q string) {
	t.Helper()
	if err := db.Exec(q).Error; err != nil {
		t.Fatalf("execSQL failed: %v\nquery: %s", err, q)
	}
}

func remigrate(t *testing.T) {
	t.Helper()
	if err := db.AutoMigrate(
		&models.Order{},
		&models.Delivery{},
		&models.Payment{},
		&models.Item{},
	).Error; err != nil {
		t.Fatalf("remigrate failed: %v", err)
	}
}

func TestErrorPaths_Coverage(t *testing.T) {
	t.Run("CreateOrUpdate_count_error", func(t *testing.T) {
		execSQL(t, `DROP TABLE IF EXISTS orders CASCADE;`)
		err := repo.CreateOrUpdate(makeOrderHeaderOnly("err-count-001"))
		if err == nil {
			t.Fatalf("expected error from Count/orders, got nil")
		}
		remigrate(t)
	})

	t.Run("CreateOrUpdate_delivery_create_error", func(t *testing.T) {
		uid := "err-delivery-01"
		if err := repo.CreateOrUpdate(makeOrderHeaderOnly(uid)); err != nil {
			t.Fatalf("prep header failed: %v", err)
		}
		execSQL(t, `DROP TABLE IF EXISTS deliveries CASCADE;`)
		o := makeOrderFull(uid, 0)
		o.Items = nil
		if o.Payment != nil {
			o.Payment = nil
		}
		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from Delivery create, got nil")
		}
		remigrate(t)
	})

	t.Run("CreateOrUpdate_payment_create_error", func(t *testing.T) {
		uid := "err-payment-01"
		if err := repo.CreateOrUpdate(makeOrderHeaderOnly(uid)); err != nil {
			t.Fatalf("prep header failed: %v", err)
		}
		execSQL(t, `DROP TABLE IF EXISTS payments CASCADE;`)
		o := makeOrderFull(uid, 0)
		o.Items = nil
		o.Delivery = nil
		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from Payment create, got nil")
		}
		remigrate(t)
	})

	t.Run("CreateOrUpdate_items_delete_error", func(t *testing.T) {
		uid := "err-items-del-01"

		if err := repo.CreateOrUpdate(makeOrderHeaderOnly(uid)); err != nil {
			t.Fatalf("prep header failed: %v", err)
		}
		execSQL(t, `DROP TABLE IF EXISTS items CASCADE;`)
		o := makeOrderFull(uid, 2)
		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from Items delete, got nil")
		}
		remigrate(t)
	})

	t.Run("Get_error", func(t *testing.T) {
		execSQL(t, `DROP TABLE IF EXISTS orders CASCADE;`)
		_, err := repo.Get("nope")
		if err == nil {
			t.Fatalf("expected error from Get with missing orders table, got nil")
		}
		remigrate(t)
	})

	t.Run("GetAll_error", func(t *testing.T) {
		execSQL(t, `DROP TABLE IF EXISTS orders CASCADE;`)
		_, err := repo.GetAll()
		if err == nil {
			t.Fatalf("expected error from GetAll with missing orders table, got nil")
		}
		remigrate(t)
	})
}

func addCheck(t *testing.T, table, cname, condition string) {
	t.Helper()
	q := fmt.Sprintf(`ALTER TABLE %s ADD CONSTRAINT %s CHECK (%s);`, table, cname, condition)
	if err := db.Exec(q).Error; err != nil {
		t.Fatalf("addCheck failed: %v\nquery: %s", err, q)
	}
}

func dropCheck(t *testing.T, table, cname string) {
	t.Helper()
	q := fmt.Sprintf(`ALTER TABLE %s DROP CONSTRAINT IF EXISTS %s;`, table, cname)
	if err := db.Exec(q).Error; err != nil {
		t.Fatalf("dropCheck failed: %v\nquery: %s", err, q)
	}
}

func remigrateClean(t *testing.T) {
	t.Helper()
	_ = db.Exec(`DROP TABLE IF EXISTS items CASCADE;`).Error
	_ = db.Exec(`DROP TABLE IF EXISTS deliveries CASCADE;`).Error
	_ = db.Exec(`DROP TABLE IF EXISTS payments CASCADE;`).Error
	_ = db.Exec(`DROP TABLE IF EXISTS orders CASCADE;`).Error

	if err := db.AutoMigrate(
		&models.Order{},
		&models.Delivery{},
		&models.Payment{},
		&models.Item{},
	).Error; err != nil {
		t.Fatalf("remigrateClean failed: %v", err)
	}
}

func TestErrorCoverage_Targeted(t *testing.T) {
	remigrateClean(t)

	t.Run("Create_header_fails_on_check", func(t *testing.T) {
		addCheck(t, "orders", "chk_hdr_uid_len", "char_length(order_uid) <= 5")
		defer dropCheck(t, "orders", "chk_hdr_uid_len")

		o := makeOrderHeaderOnly("abcdef-long")
		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from tx.Create(&hdr), got nil")
		}
	})

	t.Run("Delivery_create_fails_on_check", func(t *testing.T) {
		uid := "dlv-cr-01"
		if err := repo.CreateOrUpdate(makeOrderHeaderOnly(uid)); err != nil {
			t.Fatalf("prep order header failed: %v", err)
		}
		addCheck(t, "deliveries", "chk_city_len_le_1", "char_length(city) <= 1")
		defer dropCheck(t, "deliveries", "chk_city_len_le_1")

		o := makeOrderFull(uid, 0)
		o.Items = nil
		o.Payment = nil

		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from Delivery Create, got nil")
		}
	})

	t.Run("Delivery_update_fails_on_check", func(t *testing.T) {
		uid := "dlv-upd-01"

		o1 := makeOrderFull(uid, 0)
		o1.Items = nil
		o1.Payment = nil
		if err := repo.CreateOrUpdate(o1); err != nil {
			t.Fatalf("prep delivery initial failed: %v", err)
		}

		addCheckNotValid(t, "deliveries", "chk_city_len_le_1_u", "char_length(city) <= 1")
		defer dropCheck(t, "deliveries", "chk_city_len_le_1_u")

		o2 := makeOrderFull(uid, 0)
		o2.Items = nil
		o2.Payment = nil
		if o2.Delivery != nil {
			o2.Delivery.City = "Amsterdam"
		}
		err := repo.CreateOrUpdate(o2)
		if err == nil {
			t.Fatalf("expected error from Delivery Updates, got nil")
		}
	})

	t.Run("Payment_create_fails_on_check", func(t *testing.T) {
		uid := "pay-cr-01"
		if err := repo.CreateOrUpdate(makeOrderHeaderOnly(uid)); err != nil {
			t.Fatalf("prep order header failed: %v", err)
		}

		addCheck(t, "payments", "chk_amount_negative", "amount < 0")
		defer dropCheck(t, "payments", "chk_amount_negative")

		o := makeOrderFull(uid, 0)
		o.Items = nil
		o.Delivery = nil
		err := repo.CreateOrUpdate(o)
		if err == nil {
			t.Fatalf("expected error from Payment Create, got nil")
		}
	})

	t.Run("Payment_update_fails_on_check", func(t *testing.T) {
		uid := "pay-upd-01"

		o1 := makeOrderFull(uid, 0)
		o1.Items = nil
		o1.Delivery = nil
		if err := repo.CreateOrUpdate(o1); err != nil {
			t.Fatalf("prep payment initial failed: %v", err)
		}

		addCheckNotValid(t, "payments", "chk_amount_negative_u", "amount < 0")
		defer dropCheck(t, "payments", "chk_amount_negative_u")

		o2 := o1
		if o2.Payment != nil {
			o2.Payment.Amount = o2.Payment.Amount + 1
		}
		err := repo.CreateOrUpdate(o2)
		if err == nil {
			t.Fatalf("expected error from Payment Updates, got nil")
		}
	})

	remigrateClean(t)
}
