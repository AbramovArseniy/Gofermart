package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

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

func OrderNumIsRight(number string) bool {
	checkNumber := checksum(number)
	return checkNumber == 0
}

func checksum(number string) int {
	var luhn int
	for i := len(number) - 1; i >= 0; i-- {
		cur := (int)(number[i] - '0')
		if (len(number)-i)%2 == 0 { // even
			cur *= 2
			if cur > 9 {
				cur = cur%10 + 1
			}
		}
		luhn = (luhn + cur) % 10
	}
	return luhn % 10
}

func (c Client) DoRequest(number string) ([]byte, error) {
	resp, err := http.Get(c.URL)
	if err != nil {
		return nil, fmt.Errorf("cannot get info from accrual system: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("PostOrderHandler: error while reading response body from accrual system: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("accrual system returned status %d, error", resp.StatusCode)
	}
	return body, nil
}

func (g *Gophermart) PostOrderHandler(c echo.Context) error {
	log.Println(c.Request().Cookie("Set-Cookie"))
	log.Println(c.Request().Header.Get("Authorization"))
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		errorr := fmt.Sprintf("cannot read request body %s", err)
		http.Error(c.Response().Writer, errorr, http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while reading request body: %w", err)
	}

	orderNum := string(body)
	numIsRight := OrderNumIsRight(orderNum)
	userID, exists, err := g.Storage.GetOrderUserByNum(orderNum)
	if err != nil {
		errorr := fmt.Sprintf("cannot get user id by order number %s", err)
		http.Error(c.Response().Writer, errorr, http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while getting user id by order number: %w", err)
	}
	if !numIsRight {
		c.Response().Writer.WriteHeader(http.StatusUnprocessableEntity)
		return nil
	}
	order := Order{
		UserID: g.Auth.GetUserID(c.Request()),
		Number: orderNum,
		Status: "NEW",
	}
	if !exists {
		log.Println("user id while saving:", order.UserID)
		err = g.Storage.SaveOrder(&order)
		if err != nil {
			errorr := fmt.Sprintf("cannot save order %s", err)
			http.Error(c.Response().Writer, errorr, http.StatusInternalServerError)
			return fmt.Errorf("error while saving order: %w", err)
		}
		c.Response().Writer.WriteHeader(http.StatusAccepted)
		return err
	}

	log.Printf("id in order %d, id in req %d", userID, g.Auth.GetUserID(c.Request()))
	if userID != g.Auth.GetUserID(c.Request()) {
		err = fmt.Errorf("order already uploaded by another user")
		http.Error(c.Response().Writer, "order already uploaded by another user", http.StatusConflict)
		return err
	}
	http.Error(c.Response().Writer, "order already uploaded by you", http.StatusOK)
	return fmt.Errorf("order already uploaded by you")
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	c.Response().Header().Set("Content-Type", "application/json")
	userid := g.Auth.GetUserID(c.Request())
	orders, exist, err := g.Storage.GetOrdersByUser(userid)
	if err != nil {
		c.Response().Writer.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("GetOrdersHandler: error while getting orders by user: %w", err)
	}
	if !exist {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	var body []byte
	if body, err = json.Marshal(&orders); err != nil {
		c.Response().Writer.WriteHeader(http.StatusInternalServerError)
		return err
	}

	_, err = c.Response().Writer.Write(body)
	if err != nil {
		c.Response().Writer.WriteHeader(http.StatusInternalServerError)
		return err
	}
	return nil
}

func (g *Gophermart) PostWithdrawalHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		http.Error(c.Response().Writer, "error while reading request body", http.StatusInternalServerError)
		return fmt.Errorf("error while reading request body: %w", err)
	}
	var w Withdrawal
	err = json.Unmarshal(body, &w)
	if err != nil {
		return fmt.Errorf("error while reading request body: %w", err)
	}
	if !OrderNumIsRight(w.OrderNum) {
		http.Error(c.Response().Writer, "wrong format of order number", http.StatusUnprocessableEntity)
		return nil
	}
	balance, _, err := g.Storage.GetBalance(g.Auth.GetUserID(c.Request()))
	if err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error while counting balance: %w", err)
	}
	if balance < w.Accrual {
		http.Error(c.Response().Writer, "not enough accrual on balance", http.StatusPaymentRequired)
		return nil
	}
	g.Storage.SaveWithdrawal(w, g.Auth.GetUserID(c.Request()))
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) RegistHandler(c echo.Context) error {
	var userData UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.RegisterUser(userData)
	if err != nil && !errors.Is(err, ErrInvalidData) {
		http.Error(c.Response().Writer, "RegistHandler: can't login", http.StatusLoopDetected) // must be 500, changed to 508 for test
		return nil
	}
	if errors.Is(err, ErrInvalidData) {
		http.Error(c.Response().Writer, "RegistHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, cookie, err := g.Auth.GenerateToken(user)
	if err != nil {
		http.Error(c.Response().Writer, "RegistHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.SetCookie(&cookie)
	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetBalanceHandler(c echo.Context) error {
	var b Balance
	var err error
	c.Response().Writer.Header().Add("Content-Type", "application/json")
	b.Balance, b.Withdrawn, err = g.Storage.GetBalance(g.Auth.GetUserID(c.Request()))
	if err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error while counting balance: %w", err)
	}
	response, err := json.Marshal(b)
	if err != nil {
		http.Error(c.Response().Writer, "cannot marshal response json", http.StatusInternalServerError)
		return fmt.Errorf("error while marshling response json: %w", err)
	}
	_, err = c.Response().Writer.Write(response)
	if err != nil {
		http.Error(c.Response().Writer, "cannot write response", http.StatusInternalServerError)
		return fmt.Errorf("error while writing response: %w", err)
	}
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) AuthHandler(c echo.Context) error {
	var userData UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.LoginUser(userData)
	if err != nil && !errors.Is(err, ErrInvalidData) {
		http.Error(c.Response().Writer, "AuthHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, ErrInvalidData) {
		http.Error(c.Response().Writer, "AuthHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, _, err := g.Auth.GenerateToken(user)
	if err != nil {
		http.Error(c.Response().Writer, "AuthHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetWithdrawalsHandler(c echo.Context) error {
	c.Response().Writer.Header().Add("Content-Type", "application/json")
	w, exist, err := g.Storage.GetWithdrawalsByUser(g.Auth.GetUserID(c.Request()))
	if err != nil {
		http.Error(c.Response().Writer, "cannot get user's withdrawals", http.StatusInternalServerError)
		return fmt.Errorf("error while getting user's withdrawals: %w", err)
	}
	if !exist {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	response, err := json.Marshal(w)
	if err != nil {
		http.Error(c.Response().Writer, "cannot marshal response json", http.StatusInternalServerError)
		return fmt.Errorf("error while marshaling response json: %w", err)
	}
	_, err = c.Response().Write(response)
	if err != nil {
		http.Error(c.Response().Writer, "cannot write response", http.StatusInternalServerError)
		return fmt.Errorf("error while writing response: %w", err)
	}
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
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
