package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

type Order struct {
	UserID     int       `json:"user_id,omitempty"`
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    int64     `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Withdrawal struct {
	UserID      int
	OrderNum    string    `json:"order"`
	Accrual     int64     `json:"sum"`
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

func (g *Gophermart) CountBalance() int64 {
	return 0
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
	if g.CountBalance() < w.Accrual {
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

func (g *Gophermart) Router() *echo.Echo {
	e := echo.New()
	e.POST("/api/user/balance/withdraw", g.PostWithdrawalHandler)
	return e
}
