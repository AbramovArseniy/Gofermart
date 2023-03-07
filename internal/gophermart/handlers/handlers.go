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

type Balance struct {
	Balance   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Order struct {
	UserID     int       `json:"user_id,omitempty"`
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Withdrawal struct {
	UserID      int
	OrderNum    string    `json:"order"`
	Accrual     float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type Gophermart struct {
	Address           string
	Database          *sql.DB
	AccrualSysAddress string
	AuthenticatedUser int
}

func NewGophermart(address, accrualSysAddress string, db *sql.DB) *Gophermart {
	return &Gophermart{
		Address:           address,
		Database:          db,
		AccrualSysAddress: accrualSysAddress,
		AuthenticatedUser: 1,
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

func (g *Gophermart) CountBalance() (float64, float64, error) {
	var balance, withdrawn float64
	err := g.Database.QueryRow(`SELECT (orders.accrual_sum - withdrawals.withdrawal_sum, withdrawals.withdrawal_sum)
	 FROM (SELECT SUM(accrual) AS accrual_sum FROM orders WHERE status = PROCESSED AND user_id = $1) orders,
     (SELECT SUM(accrual) AS withdrawal_sum FROM withdrawals) withdrawals WHERE user_id = $1`, g.AuthenticatedUser).Scan(&balance, &withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot select data from database: %w", err)
	}
	return balance, withdrawn, nil
}

func (g *Gophermart) PostWithdrawalHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Println("PostWithdrawalHandler: error while reading request body")
		http.Error(c.Response().Writer, "error while reading request body", http.StatusInternalServerError)
		return fmt.Errorf("error while reading request body: %w", err)
	}
	var w Withdrawal
	err = json.Unmarshal(body, &w)
	if err != nil {
		log.Println("PostWithdrawalHandler: error while Unmarshaling request body")
		return fmt.Errorf("error while reading request body: %w", err)
	}
	if !CalculateLuhn(w.OrderNum) {
		http.Error(c.Response().Writer, "wrong format of order number", http.StatusUnprocessableEntity)
		return nil
	}
	balance, _, err := g.CountBalance()
	if err != nil {
		log.Println("error while counting balance:", err)
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error while counting balance: %w", err)
	}
	if balance < w.Accrual {
		http.Error(c.Response().Writer, "not enough accrual on balance", http.StatusPaymentRequired)
		return nil
	}
	_, err = g.Database.Exec("INSERT (user_id, order_num, accrual, created_at) INTO withdrawals VALUES ($1, $2, $3, $4)", g.AuthenticatedUser, w.OrderNum, w.Accrual, time.Now().Format(time.RFC3339))
	if err != nil {
		log.Println("PostWithdrawalHandler: error while insert data into database:", err)
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("PostWithdrawalHandler: error while insert data into database: %w", err)
	}
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetBalanceHandler(c echo.Context) error {
	var b Balance
	var err error
	b.Balance, b.Withdrawn, err = g.CountBalance()
	if err != nil {
		log.Println("error while counting balance:", err)
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error while counting balance: %w", err)
	}
	response, err := json.Marshal(b)
	if err != nil {
		log.Println("error while marshaling response json:", err)
		http.Error(c.Response().Writer, "cannot marshal response json", http.StatusInternalServerError)
		return fmt.Errorf("error while marshling response json: %w", err)
	}
	_, err = c.Response().Writer.Write(response)
	if err != nil {
		log.Println("error while writing response:", err)
		http.Error(c.Response().Writer, "cannot write response", http.StatusInternalServerError)
		return fmt.Errorf("error while writing response: %w", err)
	}
	c.Response().Writer.Header().Add("Content-Type", "application/json")
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetWithdrawalsHandler(c echo.Context) error {
	var w []Withdrawal
	rows, err := g.Database.Query(`SELECT (order_num, accrual, created_at) FROM withdrawals WHERE user_id=$1`, g.AuthenticatedUser)
	if errors.Is(err, sql.ErrNoRows) {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
	}
	if err != nil {
		log.Println("error while selecting withdrawals from database:", err)
		http.Error(c.Response().Writer, "cannot get withdrawals from database", http.StatusInternalServerError)
		return fmt.Errorf("error while selecting withdrawals from database: %w", err)
	}
	for rows.Next() {
		var withdrawal Withdrawal
		err = rows.Scan(&withdrawal.OrderNum, &withdrawal.Accrual, &withdrawal.ProcessedAt)
		if err != nil {
			log.Println("error while scanning data:", err)
			http.Error(c.Response().Writer, "cannot scan data", http.StatusInternalServerError)
			return fmt.Errorf("error while scanning data: %w", err)
		}
		w = append(w, withdrawal)
	}
	if rows.Err() != nil {
		log.Println("rows.Err:", err)
		http.Error(c.Response().Writer, "error with database rows", http.StatusInternalServerError)
		return fmt.Errorf("rows.Err: %w", err)
	}
	response, err := json.Marshal(w)
	if err != nil {
		log.Println("error while marshaling response json:", err)
		http.Error(c.Response().Writer, "cannot marshal response json", http.StatusInternalServerError)
		return fmt.Errorf("error while marshaling response json: %w", err)
	}
	_, err = c.Response().Write(response)
	if err != nil {
		log.Println("error while writing response:", err)
		http.Error(c.Response().Writer, "cannot write response", http.StatusInternalServerError)
		return fmt.Errorf("error while writing response: %w", err)
	}
	c.Response().Writer.Header().Add("Content-Type", "application/json")
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) Router() *echo.Echo {
	e := echo.New()
	e.POST("/api/user/balance/withdraw", g.PostWithdrawalHandler)
	e.GET("/api/user/balance", g.GetBalanceHandler)
	e.GET("/api/user/withdrawals", g.GetWithdrawalsHandler)
	return e
}
