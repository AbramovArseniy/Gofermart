package handlers

import (
	"net/http"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type handler struct {
	storage *storage.Storage
}

func New(storage *storage.Storage) handler {
	return handler{
		storage: storage,
	}
}

func (h handler) Route() *echo.Echo {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/api/orders/:number", h.number)
	e.POST("/api/orders", h.orders)
	e.POST("/api/goods", h.goods)

	return e
}

func (h handler) number(c echo.Context) error {
	return c.String(http.StatusOK, "number ok")
}

func (h handler) orders(c echo.Context) error {
	return c.String(http.StatusOK, "orders ok")
}

func (h handler) goods(c echo.Context) error {
	return c.String(http.StatusOK, "goods ok")
}
