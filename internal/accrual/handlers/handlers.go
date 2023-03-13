package handlers

import (
	"fmt"
	"net/http"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/services"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/storage"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type handler struct {
	Keeper storage.Keeper
}

func New(keeper storage.Keeper) handler {
	return handler{
		Keeper: keeper,
	}
}

func (h handler) Route() *echo.Echo {
	e := echo.New()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	e.GET("/api/orders/:number", h.ordersNumber)
	e.POST("/api/orders", h.orders)
	e.POST("/api/goods", h.goods)

	return e
}

func (h handler) ordersNumber(c echo.Context) error {
	_, status := services.OrderCheck(c.Param("number"))
	if status != http.StatusOK {
		c.Response().Writer.WriteHeader(status)
		err := fmt.Errorf("order not registred")
		return err
	}
	orderinfo, err := services.OrdersNumber(c.Param("number"), h.Keeper)
	if err != nil {
		return c.String(http.StatusNoContent, fmt.Sprintf("%s", err))
	}
	c.Response().Writer.Write(orderinfo)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (h handler) orders(c echo.Context) error {
	i := services.OrderAdd(c.Request().Body, h.Keeper)
	return i
}

func (h handler) goods(c echo.Context) error {
	return c.String(http.StatusOK, "goods ok")
}
