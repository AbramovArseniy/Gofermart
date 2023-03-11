package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

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

func (d Database) GetBalance(authUser User) (float64, float64, error) {
	var balance, withdrawn float64
	err := d.SelectBalacneAndWithdrawnStmt.QueryRow(authUser).Scan(&balance, &withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot select data from database: %w", err)
	}
	return balance, withdrawn, nil
}

func (d Database) GetUser(login string) (User, bool, error) {
	return User{}, false, nil
}

func (d Database) RegisterUser(user User) error {
	return nil
}

func (d Database) SaveWithdrawal(w Withdrawal, authUser User) error {
	_, err := d.InsertWirdrawalStmt.Exec(authUser, w.OrderNum, w.Accrual, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("PostWithdrawalHandler: error while insert data into database: %w", err)
	}
	return nil
}

func (d Database) GetWithdrawalsByUser(authUser User) ([]Withdrawal, bool, error) {
	var w []Withdrawal
	rows, err := d.SelectWithdrawalsByUserStmt.Query(authUser)
	if err != nil {
		log.Println("error while selecting withdrawals from database:", err)
		return nil, false, fmt.Errorf("error while selecting withdrawals from database: %w", err)
	}
	for rows.Next() {
		var withdrawal Withdrawal
		err = rows.Scan(&withdrawal.OrderNum, &withdrawal.Accrual, &withdrawal.ProcessedAt)
		if err != nil {
			log.Println("error while scanning data:", err)
			return nil, false, fmt.Errorf("error while scanning data: %w", err)
		}
		w = append(w, withdrawal)
	}
	if rows.Err() != nil {
		log.Println("rows.Err:", err)
		return nil, false, fmt.Errorf("rows.Err: %w", err)
	}
	if len(w) == 0 {
		return nil, false, nil
	}
	return w, true, nil
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
	balance, _, err := g.Storage.GetBalance(g.AuthenticatedUser)
	if err != nil {
		log.Println("error while counting balance:", err)
		http.Error(c.Response().Writer, err.Error(), http.StatusInternalServerError)
		return fmt.Errorf("error while counting balance: %w", err)
	}
	if balance < w.Accrual {
		http.Error(c.Response().Writer, "not enough accrual on balance", http.StatusPaymentRequired)
		return nil
	}
	g.Storage.SaveWithdrawal(w, g.AuthenticatedUser)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetBalanceHandler(c echo.Context) error {
	var b Balance
	var err error
	b.Balance, b.Withdrawn, err = g.Storage.GetBalance(g.AuthenticatedUser)
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
	w, exist, err := g.Storage.GetWithdrawalsByUser(g.AuthenticatedUser)
	if err != nil {
		log.Println("error while getting user's withdrawals:", err)
		http.Error(c.Response().Writer, "cannot get user's withdrawals", http.StatusInternalServerError)
		return fmt.Errorf("error while getting user's withdrawals: %w", err)
	}
	if !exist {
		c.Response().Writer.WriteHeader(http.StatusNoContent)
		return nil
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
