package handlers

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"path"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
)

type Storage interface {
	SaveOrder(authUserID int, accrualSysClient Client, order *Order) error
	SaveWithdrawal(withdrawal Withdrawal, authUserID int) error
	GetOrderUserByNum(orderNum string) (userID int, exists bool, numFormatRight bool, err error)
	GetOrdersByUser(authUserID int) (orders []Order, exist bool, err error)
	GetBalance(authUserID int) (balance float64, withdrawn float64, err error)
	GetUserData(login string) (User, error)
	RegisterNewUser(login string, password string) (User, error)
	UpgradeOrderStatus(accrualSysClient Client, orderNum string) error
	GetWithdrawalsByUser(authUserID int) (withdrawals []Withdrawal, exists bool, err error)
	SetStorage() error
	CheckOrders(accrualSysClient Client)
	Close()
}

type Withdrawal struct {
	UserID      int
	OrderNum    string    `json:"order"`
	Accrual     float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type Database struct {
	Storage
	DB                                *sql.DB
	CheckOrderInterval                time.Duration
	SelectOrdersByUserStmt            *sql.Stmt
	SelectOrderByNumStmt              *sql.Stmt
	InsertOrderStmt                   *sql.Stmt
	UpdateOrderStatusToProcessingStmt *sql.Stmt
	UpdateOrderStatusToProcessedStmt  *sql.Stmt
	UpdateOrderStatusToInvalidStmt    *sql.Stmt
	UpdateOrderStatusToUnknownStmt    *sql.Stmt
	SelectNotProcessedOrdersStmt      *sql.Stmt
	SelectBalacneAndWithdrawnStmt     *sql.Stmt
	InsertWirdrawalStmt               *sql.Stmt
	SelectWithdrawalsByUserStmt       *sql.Stmt
	InsertUserStmt                    *sql.Stmt
	SelectUserStmt                    *sql.Stmt
}

func NewDatabase(db *sql.DB) Database {
	selectOrdersByUserStmt, err := db.Prepare(`SELECT (number, status, accrual, uploaded_at) FROM orders WHERE used_id=$1`)
	if err == nil {
		log.Println("cannot prepare selectOrdersByUserStmt:", err)
	}
	selectOrderByNumStmt, err := db.Prepare(`SELECT ( status, accrual, user_id) FROM orders WHERE number=$1`)
	if err == nil {
		log.Println("cannot prepare selectOrderByNumStmt:", err)
	}
	insertOrderStmt, err := db.Prepare(`INSERT INTO orders (user_id, number, status, accrual, uploaded_at) VALUES ($1, $2, $3, $4)`)
	if err == nil {
		log.Println("cannot prepare InsertOrderStmt:", err)
	}
	updateOrderStatusToProcessingStmt, err := db.Prepare(`UPDATE orders SET status=PROCESSING WHERE order_num=$1`)
	if err == nil {
		log.Println("cannot prepare UpdateOrderStatusToProcessingStmt:", err)
	}
	updateOrderStatusToProcessedStmt, err := db.Prepare(`UPDATE orders SET status=PROCESSED, accrual=$1 WHERE order_num=$2`)
	if err == nil {
		log.Println("cannot prepare updateOrderStatusToProcessedStmt:", err)
	}
	updateOrderStatusToInvalidStmt, err := db.Prepare(`UPDATE orders SET status=INVALID WHERE order_num=$1`)
	if err == nil {
		log.Println("cannot prepare updateOrderStatusToInvalidStmt:", err)
	}
	updateOrderStatusToUnknownStmt, err := db.Prepare(`UPDATE orders SET status=UNKNOWN WHERE order_num=$1`)
	if err == nil {
		log.Println("cannot prepare updateOrderStatusToUnknownStmt:", err)
	}
	selectNotProcessedOrdersStmt, err := db.Prepare(`SELECT (order_num) FROM orders WHERE status=NEW OR status=PROCESSING`)
	if err == nil {
		log.Println("cannot prepare selectNotProcessedOrdersStmt:", err)
	}
	selectBalacneAndWithdrawnStmt, err := db.Prepare(`SELECT (orders.accrual_sum - withdrawals.withdrawal_sum, withdrawals.withdrawal_sum)
	FROM (SELECT SUM(accrual) AS accrual_sum FROM orders WHERE status = PROCESSED AND user_id = $1) orders,
	(SELECT SUM(accrual) AS withdrawal_sum FROM withdrawals) withdrawals WHERE user_id = $1`)
	if err != nil {
		log.Println("cannot prepare selectBalacneAndWithdrawnStmt:", err)
	}
	insertWirdrawal, err := db.Prepare("INSERT INTO withdrawals (user_id, order_num, accrual, created_at) VALUES ($1, $2, $3, $4)")
	if err != nil {
		log.Println("cannot prepare insertWirdrawal:", err)
	}
	selectWithdrawalsByUser, err := db.Prepare(`SELECT (order_num, accrual, created_at) FROM withdrawals WHERE user_id=$1`)
	if err != nil {
		log.Println("cannot prepare selectWithdrawalsByUser:", err)
	}
	return Database{
		DB:                                db,
		CheckOrderInterval:                5 * time.Second,
		SelectOrdersByUserStmt:            selectOrdersByUserStmt,
		SelectOrderByNumStmt:              selectOrderByNumStmt,
		InsertOrderStmt:                   insertOrderStmt,
		UpdateOrderStatusToProcessingStmt: updateOrderStatusToProcessingStmt,
		UpdateOrderStatusToProcessedStmt:  updateOrderStatusToProcessedStmt,
		UpdateOrderStatusToInvalidStmt:    updateOrderStatusToInvalidStmt,
		UpdateOrderStatusToUnknownStmt:    updateOrderStatusToUnknownStmt,
		SelectNotProcessedOrdersStmt:      selectNotProcessedOrdersStmt,
		SelectBalacneAndWithdrawnStmt:     selectBalacneAndWithdrawnStmt,
		InsertWirdrawalStmt:               insertWirdrawal,
		SelectWithdrawalsByUserStmt:       selectWithdrawalsByUser,
	}
}

type Client struct {
	URL    string
	Client http.Client
}

type Balance struct {
	Balance   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
type Order struct {
	UserID     int       `json:"user_id,omitempty"`
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Gophermart struct {
	Storage            Storage
	AccrualSysClient   Client
	AuthenticatedUser  User
	CheckOrderInterval time.Duration
	Auth               Authorization // added A
}

// уже в пакете services. не перемещать
// type User struct {
// 	Login        string
// 	HashPassword string
// 	ID           int
// }

func NewGophermart(accrualSysAddress string, db *sql.DB, auth string) *Gophermart {
	return &Gophermart{
		Storage: NewDatabase(db),
		AccrualSysClient: Client{
			URL:    path.Join(accrualSysAddress, "api/orders"),
			Client: http.Client{},
		},
		AuthenticatedUser: User{
			Login:        "",
			HashPassword: "",
			ID:           1,
		},
		CheckOrderInterval: 5 * time.Second,
		Auth:               NewAuth(auth), // added A
	}
}

func SetStorage(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/gophermart/migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("could not create migration: %w", err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func (d Database) Close() {
	d.DB.Close()
}
