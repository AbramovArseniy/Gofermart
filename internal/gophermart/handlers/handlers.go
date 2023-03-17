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
		log.Println("PostOrderHandler: error while reading response body from accrual system:", err)
		return nil, fmt.Errorf("PostOrderHandler: error while reading response body from accrual system: %w", err)
	}
	resp.Body.Close()
	if resp.StatusCode > 299 {
		return nil, fmt.Errorf("accrual system returned status %d, error", resp.StatusCode)
	}
	return body, nil
}

// func (d Database) UpgradeOrderStatus(accrualSysClient Client, orderNum string) error {
// 	body, err := accrualSysClient.DoRequest(orderNum)
// 	if err != nil {
// 		return fmt.Errorf("error while getting response body from accrual system: %w", err)
// 	}
// 	var o Order
// 	err = json.Unmarshal(body, &o)
// 	if err != nil {
// 		log.Println("failed to unmarshal json from response body from accrual system:", err)
// 		return fmt.Errorf("failed to unmarshal json from response body from accrual system: %w", err)
// 	}
// 	if o.Status == "PROCESSING" || o.Status == "REGISTERED" {
// 		_, err = d.UpdateOrderStatusToProcessingStmt.Exec(orderNum)
// 	} else if o.Status == "INVALID" {
// 		_, err = d.UpdateOrderStatusToInvalidStmt.Exec(orderNum)
// 	} else if o.Status == "PROCESSED" {
// 		_, err = d.UpdateOrderStatusToInvalidStmt.Exec(o.Accrual, orderNum)
// 	} else {
// 		_, err = d.UpdateOrderStatusToInvalidStmt.Exec(orderNum)
// 	}
// 	if err != nil {
// 		log.Println("error inserting data to db:", err)
// 		return fmt.Errorf("error inserting data to db: %w", err)
// 	}
// 	return nil
// }

// func (d Database) GetBalance(authUserID int) (float64, float64, error) {
// 	var balance, withdrawn float64
// 	err := d.SelectBalacneAndWithdrawnStmt.QueryRow(authUserID).Scan(&balance, &withdrawn)
// 	if err != nil {
// 		return 0, 0, fmt.Errorf("cannot select data from database: %w", err)
// 	}

// 	balance = balance - withdrawn

// 	return balance, withdrawn, nil
// }

// func (d Database) SaveWithdrawal(w Withdrawal, authUserID int) error {
// 	_, err := d.InsertWirdrawalStmt.Exec(authUserID, w.OrderNum, w.Accrual, time.Now().Format(time.RFC3339))
// 	if err != nil {
// 		return fmt.Errorf("PostWithdrawalHandler: error while insert data into database: %w", err)
// 	}
// 	return nil
// }

func (g *Gophermart) PostOrderHandler(c echo.Context) error {
	body, err := io.ReadAll(c.Request().Body)
	if err != nil {
		log.Println("PostOrderHandler: error while reading request body:", err)
		http.Error(c.Response().Writer, "cannot read request body", http.StatusInternalServerError)
		return fmt.Errorf("PostOrderHandler: error while reading request body: %w", err)
	}
	orderNum := fmt.Sprintf("%x", body)
	order := Order{}
	numIsRight := OrderNumIsRight(orderNum)
	userID, exists, err := g.Storage.GetOrderUserByNum(orderNum)
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
		err = g.Storage.SaveOrder(g.Auth.GetUserID(c.Request()), &order)
		if err != nil {
			log.Println("error while saving order:", err)
			http.Error(c.Response().Writer, "cannot save order", http.StatusInternalServerError)
			return fmt.Errorf("error while saving order: %w", err)
		}
	}
	if order.UserID == g.Auth.GetUserID(c.Request()) {
		c.Response().Writer.WriteHeader(http.StatusOK)
		return nil
	} else {
		http.Error(c.Response().Writer, "order already uploaded by another user", http.StatusConflict)
		return nil
	}
}

func (g *Gophermart) GetOrdersHandler(c echo.Context) error {
	authuserID := g.Auth.GetUserID(c.Request())
	log.Printf("authuserID: %+v", authuserID)
	orders, exist, err := g.Storage.GetOrdersByUser(authuserID)
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
	return nil
}

// func (d Database) GetWithdrawalsByUser(authUserID int) ([]Withdrawal, bool, error) {
// 	var w []Withdrawal
// 	rows, err := d.SelectWithdrawalsByUserStmt.Query(authUserID)
// 	if err != nil {
// 		log.Println("error while selecting withdrawals from database:", err)
// 		return nil, false, fmt.Errorf("error while selecting withdrawals from database: %w", err)
// 	}
// 	for rows.Next() {
// 		var withdrawal Withdrawal
// 		err = rows.Scan(&withdrawal.OrderNum, &withdrawal.Accrual, &withdrawal.ProcessedAt)
// 		if err != nil {
// 			log.Println("error while scanning data:", err)
// 			return nil, false, fmt.Errorf("error while scanning data: %w", err)
// 		}
// 		w = append(w, withdrawal)
// 	}
// 	if rows.Err() != nil {
// 		log.Println("rows.Err:", err)
// 		return nil, false, fmt.Errorf("rows.Err: %w", err)
// 	}
// 	if len(w) == 0 {
// 		return nil, false, nil
// 	}
// 	return w, true, nil
// }

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
	if !OrderNumIsRight(w.OrderNum) {
		http.Error(c.Response().Writer, "wrong format of order number", http.StatusUnprocessableEntity)
		return nil
	}
	balance, _, err := g.Storage.GetBalance(g.Auth.GetUserID(c.Request()))
	if err != nil {
		log.Println("error while counting balance:", err)
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
	log.Printf("RegistHandler - userdata: %+v", userData)
	user, err := g.Auth.RegisterUser(userData)
	if err != nil && !errors.Is(err, ErrInvalidData) {
		log.Printf("RegistHandler: error while REGISTER: %v", err)
		http.Error(c.Response().Writer, "RegistHandler: can't login", http.StatusLoopDetected) // must be 500, changed to 508 for test
		return nil
	}
	if errors.Is(err, ErrInvalidData) {
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

func (g *Gophermart) GetBalanceHandler(c echo.Context) error {
	var b Balance
	var err error
	b.Balance, b.Withdrawn, err = g.Storage.GetBalance(g.Auth.GetUserID(c.Request()))
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
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, ErrInvalidData) {
		http.Error(c.Response().Writer, "AuthHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, err := g.Auth.GenerateToken(user)
	if err != nil {
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) GetWithdrawalsHandler(c echo.Context) error {
	w, exist, err := g.Storage.GetWithdrawalsByUser(g.Auth.GetUserID(c.Request()))
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

// func (d Database) CheckOrders(accrualSysClient Client) {
// 	ticker := time.NewTicker(d.CheckOrderInterval)
// 	for {
// 		<-ticker.C
// 		rows, err := d.SelectNotProcessedOrdersStmt.Query()
// 		if errors.Is(err, sql.ErrNoRows) {
// 			return
// 		}
// 		if err != nil {
// 			log.Println("CheckOrders: error while selecting data from Database")
// 			return
// 		}
// 		for rows.Next() {
// 			var orderNum string
// 			rows.Scan(&orderNum)
// 			d.UpgradeOrderStatus(accrualSysClient, orderNum)
// 		}
// 		if rows.Err() != nil {
// 			log.Println("CheckOrders: error while reading rows")
// 		}
// 	}

// }

// func (d Database) RegisterNewUser(login string, password string) (User, error) {
// 	row := d.InsertUserStmt.QueryRowContext(context.Background(), login, password)
// 	var user User
// 	err := row.Scan(&user.ID)
// 	var pgErr *pgconn.PgError
// 	if errors.As(err, &pgErr) {
// 		if pgErr.Code == pgerrcode.UniqueViolation {
// 			return User{}, ErrUserExists
// 		}
// 	}
// 	return user, nil
// }

// func (d Database) GetUserData(login string) (User, error) {
// 	var user User
// 	row := d.SelectUserStmt.QueryRow(login)
// 	err := row.Scan(&user.ID, &user.Login, &user.HashPassword)
// 	if err == pgx.ErrNoRows {
// 		return User{}, nil
// 	}
// 	return user, err
// }

func (g *Gophermart) Router() *echo.Echo {

	e := echo.New()
	e.Use(middleware.Logger())
	e.POST("/api/user/register", g.RegistHandler)
	e.POST("/api/user/login", g.AuthHandler)

	logged := e.Group("/api/user", echojwt.JWT([]byte(g.secret)))
	logged.POST("/orders", g.PostOrderHandler)
	logged.GET("/orders", g.GetOrdersHandler)
	logged.POST("/balance/withdraw", g.PostWithdrawalHandler)
	logged.GET("/balance", g.GetBalanceHandler)
	logged.GET("/withdrawals", g.GetWithdrawalsHandler)
	return e
}
