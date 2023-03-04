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

	"github.com/labstack/echo/v4"
)

type Order struct {
	User_id    int       `json:"user_id,omitempty"`
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Gophermart struct {
	Address            string
	Database           *sql.DB
	AccrualSysAddress  string
	AuthenticatedUser  int
	CheckOrderInterval time.Duration
}

func NewGophermart(address, accrualSysAddress string, db *sql.DB) *Gophermart {
	return &Gophermart{
		Address:            address,
		Database:           db,
		AccrualSysAddress:  accrualSysAddress,
		AuthenticatedUser:  1,
		CheckOrderInterval: 5 * time.Second,
	}
}

func CalculateLuhn(number string) bool {
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

func (g *Gophermart) UpgradeOrderStatus(orderNum string) error {
	url := fmt.Sprintf("http://%s/api/orders/%s", g.AccrualSysAddress, orderNum)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("cannot get info from accrual system: %w", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("PostOrderHandler: error while reading response body from accrual system:", err)
		return fmt.Errorf("PostOrderHandler: error while reading response body from accrual system: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil
	}
	var o Order
	err = json.Unmarshal(body, &o)
	if err != nil {
		log.Println("failed to unmarshal json from response body from accrual system:", err)
		return fmt.Errorf("failed to unmarshal json from response body from accrual system: %w", err)
	}
	if o.Status == "PROCESSING" || o.Status == "REGISTERED" {
		_, err = g.Database.Exec(`UPDATE orders SET status=PROCESSING WHERE order_num=$1`, orderNum)
	} else if o.Status == "INVALID" {
		_, err = g.Database.Exec(`UPDATE orders SET status=INVALID WHERE order_num=$1`, orderNum)
	} else if o.Status == "PROCESSED" {
		_, err = g.Database.Exec(`UPDATE orders SET status=PROCESSING, accrual=$1 WHERE order_num=$2`, o.Accrual, orderNum)
	} else {
		_, err = g.Database.Exec(`UPDATE orders SET status=UNKNOWN WHERE order_num=$1`, orderNum)
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
	if CalculateLuhn(orderNum) {
		http.Error(c.Response().Writer, "wrong format of order number", http.StatusUnprocessableEntity)
		return nil
	}
	var status string
	var accrual, userID int
	err = g.Database.QueryRow(`SELECT ( status, accrual, user_id) FROM orders WHERE number=$1`, orderNum).Scan(&status, &accrual, &userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("error reading rows from db:", err)
		http.Error(c.Response().Writer, "cannot read rows from database", http.StatusInternalServerError)
		return fmt.Errorf("error reading rows from db: %w", err)
	}
	if errors.Is(err, sql.ErrNoRows) {
		_, err = g.Database.Exec(`INSERT INTO orders (user_id, number, status, accrual, uploaded_at) VALUES ($1, $2, $3, $4)`, g.AuthenticatedUser, orderNum, "NEW", time.Now().Format(time.RFC3339))
		if err != nil {
			log.Println("error inserting data to db:", err)
			http.Error(c.Response().Writer, "cannot intert data into database", http.StatusInternalServerError)
			return fmt.Errorf("error inserting data to db: %w", err)
		}
		err = g.UpgradeOrderStatus(orderNum)
		if err != nil {
			log.Println("error while upgrading order status:", err)
			http.Error(c.Response().Writer, "cannot upgradeorder status ", http.StatusInternalServerError)
			return err
		}
	}
	if userID == g.AuthenticatedUser {
		c.Response().Writer.WriteHeader(http.StatusOK)
		return nil
	} else {
		http.Error(c.Response().Writer, "order already uploaded by another user", http.StatusConflict)
		return nil
	}
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	rows, err := g.Database.Query(`SELECT (number, status, e-ball, uploaded_at) FROM orders WHERE used_id=$1`, g.AuthenticatedUser)
	if errors.Is(err, sql.ErrNoRows) {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if err != nil {
		log.Println("GetOrdersHandler: error while getting orders from database:", err)
		http.Error(c.Response().Writer, "cannot read data from database", http.StatusInternalServerError)
		return err
	}
	var orders []Order
	for rows.Next() {
		var order Order
		err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			log.Println("GetOrdersHandler: error while scanning rows from database:", err)
			http.Error(c.Response().Writer, "cannot read data from database", http.StatusInternalServerError)
			return err
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		log.Println("GetOrdersHandler: rows.Err() error database:", err)
		http.Error(c.Response().Writer, "cannot read data from database", http.StatusInternalServerError)
		return err
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

func (g *Gophermart) CheckOrders() {
	ticker := time.NewTicker(g.CheckOrderInterval)
	for {
		<-ticker.C
		rows, err := g.Database.Query(`SELECT (order_num) FROM orders WHERE status=NEW OR status=PROCESSING`)
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
			g.UpgradeOrderStatus(orderNum)
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
	return e
}
