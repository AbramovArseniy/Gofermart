package handlers

import (
	"database/sql"
	"encoding/json"

	//"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/storage"
	"github.com/labstack/echo/v4"
)

type Order struct {
	UserID     int       `json:"user_id,omitempty"`
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Gophermart struct {
	Address           string
	Database          *sql.DB
	AccrualSysAddress string
	AuthenticatedUser int
	Storage           storage.Storage
}

func NewGophermart(address, accrualSysAddress string, db *sql.DB) *Gophermart {
	return &Gophermart{
		Address:           address,
		Database:          db,
		AccrualSysAddress: accrualSysAddress,
		AuthenticatedUser: 1,
		Storage: storage.Storage{
			Users: []storage.User{{
				ID: 1,
			},
				{
					ID: 2,
				},
			},
			Orders: []storage.Order{
				{
					UserID:  1,
					Number:  12345678903,
					Status:  "PROCESSED",
					Accrual: 500,
				},
				{
					UserID: 2,
					Number: 9278923470,
					Status: "PROCESSING",
				},
			},
		},
	}
}

func CalculateLuhn(number int64) bool {
	checkNumber := checksum(number)
	return checkNumber == 0
}

func checksum(number int64) int64 {
	var luhn int64
	for i := 0; number > 0; i++ {
		cur := number % 10
		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}
		luhn += cur
		number = number / 10
	}
	return luhn % 10
}

func (g *Gophermart) PostOrderHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Println("PostOrderHandler: error while reading request body:", err)
		http.Error(c.Response().Writer, "cannot read request body", http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while reading request body: %w", err)
	}
	var orderNum int64
	orderNum, err = strconv.ParseInt(string(body), 10, 64)
	if err != nil {
		log.Println("PostOrderHandler: error while converting order number to int:", err)
		http.Error(c.Response().Writer, "wrong format of request", http.StatusBadRequest)
		return fmt.Errorf("PostOrderHandler: error while converting order number to int: %w", err)
	}
	if CalculateLuhn(orderNum) {
		log.Println("PostOrderHandler: wrong format of order number")
		http.Error(c.Response().Writer, "wrong format of order number", http.StatusUnprocessableEntity)
		return nil
	}
	order := g.Storage.GetOrderByNumber(orderNum)
	/*err = g.Database.QueryRow(`SELECT ( status, e-ball, user_id) FROM orders WHERE number=$1`, orderNum).Scan(&status, &accrual, &userID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Println("error reading rows from db:", err)
		http.Error(c.Response().Writer, "cannot read rows from database", http.StatusInternalServerError)
		return fmt.Errorf("error reading rows from db: %w", err)
	}*/

	if /*errors.Is(err, sql.ErrNoRows)*/ order == nil {
		/*url := fmt.Sprintf("http://%s/api/orders/%d", g.AccrualSysAddress, orderNum)
		resp, err := http.Get(url)
		if err != nil {
			http.Error(c.Response().Writer, "cannot get info from accrual system", http.StatusInternalServerError)
			return fmt.Errorf("cannot get info from accrual system: %w", err)
		}
		body, err = io.ReadAll(resp.Body)
		if err != nil {
			log.Println("PostOrderHandler: error while reading response body from accrual system:", err)
			http.Error(c.Response().Writer, "cannot read response body from accrual system", http.StatusInternalServerError)
			return fmt.Errorf("PostOrderHandler: error while reading response body from accrual system: %w", err)
		}
		resp.Body.Close()
		if resp.StatusCode > 299 {
			http.Error(c.Response().Writer, fmt.Sprintf("Response failed with status code: %d and\nbody: %s\n", resp.StatusCode, body), http.StatusInternalServerError)
			return nil
		}
		err = json.Unmarshal(body, order)*/
		order = &storage.Order{
			Number:  346436439,
			Status:  "PROCESSED",
			Accrual: 500,
		}
		/*if err != nil {
			log.Println("failed to unmarshal json from response body from accrual system:", err)
			http.Error(c.Response().Writer, "failed to unmarshal json from response body from accrual system", http.StatusInternalServerError)
			return fmt.Errorf("failed to unmarshal json from response body from accrual system: %w", err)
		}
		_, err = g.Database.Exec(`INSERT INTO orders (user_id, number, status, e-ball, uploaded_at) VALUES ($1, $2, $3, $4, $5)`, g.AuthenticatedUser, orderNum, status, accrual, time.Now().Format(time.RFC3339))
		if err != nil {
			log.Println("error inserting data to db:", err)
			http.Error(c.Response().Writer, "cannot insert data to database", http.StatusInternalServerError)
			return fmt.Errorf("error inserting data to db: %w", err)
		}*/
		order.UserID = g.AuthenticatedUser
		g.Storage.AppendOrder(order)
		c.Response().Writer.WriteHeader(http.StatusAccepted)
		return nil
	}
	if order.UserID == g.AuthenticatedUser {
		c.Response().Writer.WriteHeader(http.StatusOK)
		return nil
	} else {
		http.Error(c.Response().Writer, "order already uploaded by another user", http.StatusConflict)
		return nil
	}
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	/*rows, err := g.Database.Query(`SELECT (number, status, e-ball, uploaded_at) FROM orders WHERE used_id=$1`, g.AuthenticatedUser)*/
	orders := g.Storage.GetOrdersByUser(g.AuthenticatedUser)
	if /*errors.Is(err, sql.ErrNoRows)*/ len(orders) == 0 {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	/*if err != nil {
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
	}*/
	var body []byte
	body, err := json.MarshalIndent(&orders, "", " ")
	if err != nil {
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

func (g *Gophermart) Route() *echo.Echo {
	e := echo.New()
	e.POST("/api/user/orders", g.PostOrderHandler)
	e.GET("/api/user/orders", g.GetOrdersHandler)
	return e
}
