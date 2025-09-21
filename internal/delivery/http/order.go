package http

import (
	"errors"
	"net/http"
	"strings"

	"l0-demo/internal/repository/cache"
	"l0-demo/internal/service"

	"github.com/gin-gonic/gin"
)

// GetOrderById
// @Summary GetOrderById
// @Description Allows to get specific order from the app's cache via its uid
// @ID get-order-by-id
// @Accept json
// @Produce json
// @Param uid path string true "order's uid" minlength(19)  maxlength(19)
// @Success 200 {object} models.Order
// @Failure 400,404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Failure default {object} errorResponse
// @Router /api/order/{uid} [get]

func (h *Handler) GetOrderById(c *gin.Context) {
	uid := strings.TrimSpace(c.Param("uid"))
	if uid == "" {
		newErrorResponse(c, http.StatusBadRequest, "invalid uid")
		return
	}

	order, err := h.svc.GetCachedOrder(uid)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			newErrorResponse(c, http.StatusNotFound, "not found")
			return
		}
		if val, ok := err.(cache.ErrorHandler); ok {
			newErrorResponse(c, val.StatusCode, err.Error())
			return
		}
		newErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetDbOrderById
// @Summary GetDbOrderById
// @Description Allows to get specific order from the postgres database via its uid
// @ID get-db-order-by-id
// @Accept json
// @Produce json
// @Param uid path string true "order's uid" minlength(19)  maxlength(19)
// @Success 200 {object} models.Order
// @Failure 400,404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Failure default {object} errorResponse
// @Router /api/order/db/{uid} [get]
func (h *Handler) GetDbOrderById(c *gin.Context) {
	uid := strings.TrimSpace(c.Param("uid"))
	if uid == "" {
		newErrorResponse(c, http.StatusBadRequest, "missing uid")
		return
	}

	order, err := h.svc.GetDbOrder(uid)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			newErrorResponse(c, http.StatusNotFound, "order not found")
			return
		}
		newErrorResponse(c, http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, order)
}

// GetAllOrders
// @Summary GetAllOrders
// @Description Allows to get all orders from the app's cache
// @ID get-all-orders
// @Accept json
// @Produce json
// @Success 200 {object} getAllOrdersResponse
// @Failure 400,404 {object} errorResponse
// @Failure 500 {object} errorResponse
// @Failure default {object} errorResponse
// @Router /api/orders [get]
func (h *Handler) GetAllOrders(c *gin.Context) {
	orders, err := h.svc.GetAllCachedOrders()
	if err != nil {
		if val, ok := err.(cache.ErrorHandler); ok {
			newErrorResponse(c, val.StatusCode, err.Error())
			return
		} else {
			newErrorResponse(c, http.StatusInternalServerError, err.Error())
			return
		}
	}
	c.JSON(http.StatusOK, getAllOrdersResponse{
		Data: orders,
	})
}
