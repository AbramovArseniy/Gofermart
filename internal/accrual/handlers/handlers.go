package handlers

import (
	"fmt"
	"log"
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

	e.GET("/api/orders/:number", h.ordersChecker)
	e.POST("/api/orders", h.ordersRegister)
	e.POST("/api/goods", h.addNewGoods)

	return e
}

func (h handler) ordersChecker(c echo.Context) error {
	orderinfo, status := services.OrderCheck(c.Param("number"))
	if status != http.StatusOK {
		c.Response().Writer.WriteHeader(status)
		err := fmt.Errorf("httpstatus is not OK")
		return err
	}
	if orderinfo != nil {
		c.Response().Writer.Write(orderinfo)
		c.Response().Writer.WriteHeader(status)
		err := fmt.Errorf("order status invalid")
		return err
	}

	if !h.Keeper.FindOrder(c.Param("number")) {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
	}

	orderinfo, err := services.OrdersNumber(c.Param("number"), h.Keeper)
	if err != nil {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return err
	}
	c.Response().Writer.Write(orderinfo)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (h handler) ordersRegister(c echo.Context) error {
	httpStatus, err := services.OrderAdd(c.Request().Body, h.Keeper)
	log.Print(httpStatus)
	c.Response().Writer.WriteHeader(httpStatus)
	return err
}

func (h handler) addNewGoods(c echo.Context) error {
	httpStatus, err := services.GoodsAdd(c.Request().Body, h.Keeper)
	log.Print(httpStatus)
	c.Response().Writer.WriteHeader(httpStatus)
	return err
}
