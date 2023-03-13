package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/services"
	"github.com/labstack/echo/v4"
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
	resp, err := http.Get(c.Url)
	if err != nil {
		return nil, fmt.Errorf("cannot get info from accrual system: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("PostOrderHandler: error while reading response body from accrual system:", err)
		return nil, fmt.Errorf("PostOrderHandler: error while reading response body from accrual system: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("accrual system returned status %d, error", resp.StatusCode)
	}
	return body, nil
}

func (db Database) UpgradeOrderStatus(accrualSysClient Client, orderNum string) error {
	body, err := accrualSysClient.DoRequest(orderNum)
	if err != nil {
		return fmt.Errorf("error while getting response body from accrual system: %w", err)
	}
	var o Order
	err = json.Unmarshal(body, &o)
	if err != nil {
		log.Println("failed to unmarshal json from response body from accrual system:", err)
		return fmt.Errorf("failed to unmarshal json from response body from accrual system: %w", err)
	}
	if o.Status == "PROCESSING" || o.Status == "REGISTERED" {
		_, err = db.UpdateOrderStatusToProcessingStmt.Exec(orderNum)
	} else if o.Status == "INVALID" {
		_, err = db.UpdateOrderStatusToInvalidStmt.Exec(orderNum)
	} else if o.Status == "PROCESSED" {
		_, err = db.UpdateOrderStatusToInvalidStmt.Exec(o.Accrual, orderNum)
	} else {
		_, err = db.UpdateOrderStatusToInvalidStmt.Exec(orderNum)
	}
	if err != nil {
		log.Println("error inserting data to db:", err)
		return fmt.Errorf("error inserting data to db: %w", err)
	}
	return nil
}

func (g *Gophermart) PostOrderHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Println("PostOrderHandler: error while reading request body:", err)
		http.Error(c.Response().Writer, "cannot read request body", http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while reading request body: %w", err)
	}
	orderNum := string(body)
	if err != nil {
		http.Error(c.Response().Writer, "wrong format of request", http.StatusBadRequest)
		return fmt.Errorf("PostOrderHandler: error while converting order number to int: %w", err)
	}
	order := Order{}
	userID, exists, numIsRight, err := g.Storage.GetOrderUserByNum(orderNum)
	order.UserID = userID
	if err != nil {
		log.Println("PostOrderHandler: error while getting user id by order number:", err)
		http.Error(c.Response().Writer, "cannot get user id by order number", http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while getting user id by order number: %w", err)
	}
	if !numIsRight {
		c.Response().Writer.WriteHeader(http.StatusUnprocessableEntity)
		return nil
	}
	if !exists {
		err = g.Storage.SaveOrder(c.Request().Context().Value(userContextKey).(services.User), g.AccrualSysClient, &order)
		if err != nil {
			log.Println("error while saving order:", err)
			http.Error(c.Response().Writer, "cannot save order", http.StatusInternalServerError)
			return fmt.Errorf("error while saving order: %w", err)
		}
	}
	if order.UserID == c.Request().Context().Value(userContextKey).(services.User).ID {
		c.Response().Writer.WriteHeader(http.StatusOK)
		return nil
	} else {
		http.Error(c.Response().Writer, "order already uploaded by another user", http.StatusConflict)
		return nil
	}
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	orders, exist, err := g.Storage.GetOrdersByUser(c.Request().Context().Value(userContextKey).(services.User))
	if err != nil {
		log.Println("GetOrdersHandler: error while getting orders by user:", err)
		return fmt.Errorf("GetOrdersHandler: error while getting orders by user: %w", err)
	}
	if !exist {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	var body []byte
	if body, err = json.Marshal(&orders); err != nil {
		log.Println("GetOrdersHandler: error while marshalling json:", err)
		http.Error(c.Response().Writer, "cannot marshal data to json", http.StatusInternalServerError)
		return err
	}

	_, err = c.Response().Writer.Write(body)
	if err != nil {
		log.Println("GetOrdersHandler: error while writing response body:", err)
		http.Error(c.Response().Writer, "cannot write response body", http.StatusInternalServerError)
		return err
	}
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) RegistHandler(c echo.Context) error {
	var userData services.UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.RegisterUser(userData)
	if err != nil && !errors.Is(err, services.ErrInvalidData) {
		log.Printf("RegistHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "RegistHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, services.ErrInvalidData) {
		http.Error(c.Response().Writer, "RegistHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, err := g.Auth.GenerateToken(user)
	if err != nil {
		log.Printf("RegistHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "RegistHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) AuthHandler(c echo.Context) error {
	var userData services.UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.LoginUser(userData)
	if err != nil && !errors.Is(err, services.ErrInvalidData) {
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, services.ErrInvalidData) {
		http.Error(c.Response().Writer, "AuthHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, err := g.Auth.GenerateToken(user)
	if err != nil {
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (db Database) CheckOrders(accrualSysClient Client) {
	ticker := time.NewTicker(db.CheckOrderInterval)
	for {
		<-ticker.C
		rows, err := db.SelectNotProcessedOrdersStmt.Query()
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		if err != nil {
			log.Println("CheckOrders: error while selecting data from Database")
			return
		}
		for rows.Next() {
			var orderNum string
			rows.Scan(&orderNum)
			db.UpgradeOrderStatus(accrualSysClient, orderNum)
		}
		if rows.Err() != nil {
			log.Println("CheckOrders: error while reading rows")
		}
	}

}

func (g *Gophermart) Router() *echo.Echo {
	e := echo.New()
	e.POST("/api/user/orders", g.PostOrderHandler)
	e.GET("/api/user/orders", g.GetOrdersHandler)
	e.POST("/api/user/register", g.RegistHandler)
	e.POST("/api/user/login", g.AuthHandler)

	return e
}
