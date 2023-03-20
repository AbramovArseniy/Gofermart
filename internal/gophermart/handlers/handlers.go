package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/services"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/database"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
	echojwt "github.com/labstack/echo-jwt/v4"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

const (
	IntSymbols     = "0123456789"
	ShortURLMaxLen = 7
	userIDCookie   = "useridcookie"
	userContextKey = "user"
)

type Gophermart struct {
	Storage            types.Storage
	AccrualSysClient   types.Client
	AuthenticatedUser  types.User
	CheckOrderInterval time.Duration
	Auth               types.Authorization // added A
	secret             string
}

func NewGophermart(accrualSysAddress, secret string, database *database.DataBase, auth *AuthJWT) *Gophermart {
	accrualAddr, _ := url.Parse(accrualSysAddress)
	accrualAddr.Path = "api/orders"
	return &Gophermart{
		Storage: database,
		AccrualSysClient: types.Client{
			URL:    *accrualAddr,
			Client: http.Client{},
		},
		AuthenticatedUser: types.User{
			Login:        "",
			HashPassword: "",
			ID:           1,
		},
		CheckOrderInterval: 5 * time.Second,
		Auth:               auth,
		secret:             secret,
	}
}

func (g *Gophermart) DoRequest(number string) ([]byte, error) {

	resp, err := http.Get(g.AccrualSysClient.URL.String())
	if err != nil {
		return nil, fmt.Errorf("DoRequest: cannot get info from accrual system: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("DoRequest: error while reading response body from accrual system: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("DoRequest: accrual system returned status %d, error", resp.StatusCode)
	}
	return body, nil
}

func (g *Gophermart) RegistHandler(c echo.Context) error {
	httpStatus, token, err := services.RegistService(c.Request(), g.Auth)

	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(httpStatus)

	return err
}

func (g *Gophermart) AuthHandler(c echo.Context) error {
	httpStatus, token, err := services.AuthService(c.Request(), g.Storage, g.Auth)
	c.Response().Header().Set("Authorization", token)
	c.Response().Writer.WriteHeader(httpStatus)
	return err
}

func (g *Gophermart) PostOrderHandler(c echo.Context) error {
	httpStatus, err := services.PostOrderService(c.Request(), g.Storage, g.Auth, g.AccrualSysClient)

	c.Response().WriteHeader(httpStatus)

	return err
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "application/json")

	httpStatus, body, err := services.GetOrderService(c.Request(), g.Storage, g.Auth)
	// log.Println("GetOrdersHandler: EVERYTHING still is OK #5")
	c.Response().Writer.WriteHeader(httpStatus)
	c.Response().Writer.Write(body)

	return err
}

func (g *Gophermart) PostWithdrawalHandler(c echo.Context) error {
	httpStatus, err := services.PostWithdrawalService(c.Request(), g.Storage, g.Auth)

	c.Response().Writer.WriteHeader(httpStatus)

	return err
}

func (g *Gophermart) GetBalanceHandler(c echo.Context) error {
	c.Response().Writer.Header().Add("Content-Type", "application/json")

	httpStatus, response, err := services.GetBalanceService(c.Request(), g.Storage, g.Auth)

	c.Response().Writer.WriteHeader(httpStatus)
	c.Response().Writer.Write(response)

	return err
}

func (g *Gophermart) GetWithdrawalsHandler(c echo.Context) error {
	c.Response().Writer.Header().Add("Content-Type", "application/json")

	httpStatus, response, err := services.GetWithdrawalsService(c.Request(), g.Storage, g.Auth)

	c.Response().Writer.WriteHeader(httpStatus)
	c.Response().Write(response)

	return err
}

func (g *Gophermart) Router() *echo.Echo {
	e := echo.New()

	e.Use(middleware.LoggerWithConfig(middleware.LoggerConfig{
		Format: "time=${time_rfc3339}, method=${method}, uri=${uri}, status=${status}, error=${error}\n",
	}))
	e.POST("/api/user/register", g.RegistHandler)
	e.POST("/api/user/login", g.AuthHandler)

	logged := e.Group("/api/user", echojwt.WithConfig(echojwt.Config{SigningKey: []byte(g.secret)}))

	logged.POST("/orders", g.PostOrderHandler)
	logged.GET("/orders", g.GetOrdersHandler)
	logged.POST("/balance/withdraw", g.PostWithdrawalHandler)
	logged.GET("/balance", g.GetBalanceHandler)
	logged.GET("/withdrawals", g.GetWithdrawalsHandler)
	return e
}
