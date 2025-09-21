package http_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	httpdelivery "l0-demo/internal/delivery/http"
	"l0-demo/internal/models"
)

func fakeOrder(f *gofakeit.Faker) models.Order {
	uid := f.UUID()
	track := f.LetterN(12)
	return models.Order{
		OrderUid:    uid,
		TrackNumber: track,
		Entry:       "WBIL",
		Locale:      "en",

		InternalSignature: "",
		CustomerId:        f.Username(),
		DeliveryService:   f.RandomString([]string{"meest", "dpd", "dhl"}),
		ShardKey:          f.DigitN(1),
		SmId:              int(f.Number(1, 999)),
		DateCreated:       time.Now().UTC(),
		OofShard:          f.DigitN(1),

		Delivery: &models.Delivery{
			Name:    f.Name(),
			Phone:   f.Phone(),
			Zip:     f.Zip(),
			City:    f.City(),
			Address: f.Street(),
			Region:  f.State(),
			Email:   f.Email(),
		},
		Payment: &models.Payment{
			Transaction:  uid,
			RequestId:    "",
			Currency:     f.RandomString([]string{"USD", "EUR"}),
			Provider:     "wbpay",
			Amount:       int(f.Number(100, 5000)),
			PaymentDt:    int(f.Int64()),
			Bank:         f.Company(),
			DeliveryCost: int(f.Number(100, 2000)),
			GoodsTotal:   int(f.Number(50, 500)),
			CustomFee:    0,
		},
		Items: []models.Item{
			{
				ChrtId:      int(f.Number(1_000_000, 9_999_999)),
				TrackNumber: track,
				Price:       int(f.Number(100, 1000)),
				Rid:         f.UUID(),
				Name:        f.ProductName(),
				Sale:        int(f.Number(0, 50)),
				Size:        "0",
				TotalPrice:  int(f.Number(50, 500)),
				NmId:        int(f.Number(1_000_000, 9_999_999)),
				Brand:       f.Company(),
				Status:      202,
			},
		},
	}
}

func Test_GetAllOrders_Many(t *testing.T) {
	f := gofakeit.New(42)
	var orders []models.Order
	for i := 0; i < 20; i++ {
		orders = append(orders, fakeOrder(f))
	}

	s := &svcStub{
		getAllCached: func() ([]models.Order, error) { return orders, nil },
	}
	h := httpdelivery.NewHandler(s)
	r := h.InitRoutes()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Data []models.Order `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Data, len(orders))
}
