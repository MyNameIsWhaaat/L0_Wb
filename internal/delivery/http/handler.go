package http

import (
	"l0-demo/internal/models"
	"l0-demo/internal/service"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "l0-demo/docs"

	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Handler struct {
	svc service.Order
}

func NewHandler(s service.Order) *Handler {
	return &Handler{svc: s}
}

type getAllOrdersResponse struct {
	Data []models.Order `json:"data"`
}

func (h *Handler) InitRoutes() *gin.Engine {
	router := gin.Default()

	api := router.Group("/api")
	{
		api.GET("/order/:uid", h.GetOrderById)
		api.GET("/order/db/:uid", h.GetDbOrderById)
		api.GET("/orders", h.GetAllOrders)
	}

	router.GET("/", func(c *gin.Context) {
		c.File("internal/web/index.html")
	})

	router.Static("/web", "./web")

	router.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/api/") {
			c.JSON(http.StatusNotFound, gin.H{"message": "not found"})
			return
		}
		c.File("internal/web/index.html")
	})

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	return router
}