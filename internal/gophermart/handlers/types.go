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
	SaveOrder(authUser User, accrualSysClient Client, order *Order) error
	SaveWithdrawal(withdrawal Withdrawal) error
	GetOrdersByUser(authUser User) (orders []Order, exist bool, err error)
	GetOrderUserByNum(orderNum string) (userID int, exists bool, numIsRight bool, err error)
	GetBalance(authUser User) (balance float64, withdrawn float64, err error)
	GetUser(login string) (user User, exists bool, err error)
	RegisterUser(user User) error
	UpgradeOrderStatus(accrualSysClient Client, orderNum string) error
	SetStorage() error
	CheckOrders(accrualSysClient Client)
	Finish()
}

type Withdrawal struct {
	UserID      int
	OrderNum    string
	Accrual     float64
	ProcessedAt time.Time
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
	return Database{
		DB:                                db,
		SelectOrdersByUserStmt:            selectOrdersByUserStmt,
		SelectOrderByNumStmt:              selectOrderByNumStmt,
		InsertOrderStmt:                   insertOrderStmt,
		UpdateOrderStatusToProcessingStmt: updateOrderStatusToProcessingStmt,
		UpdateOrderStatusToProcessedStmt:  updateOrderStatusToProcessedStmt,
		UpdateOrderStatusToInvalidStmt:    updateOrderStatusToInvalidStmt,
		UpdateOrderStatusToUnknownStmt:    updateOrderStatusToUnknownStmt,
		SelectNotProcessedOrdersStmt:      selectNotProcessedOrdersStmt,
	}
}

type Client struct {
	Url    string
	Client http.Client
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
}

type User struct {
	Login        string
	HashPassword string
	ID           int
}

func NewGophermart(accrualSysAddress string, db *sql.DB) *Gophermart {
	return &Gophermart{
		Storage: NewDatabase(db),
		AccrualSysClient: Client{
			Url:    path.Join(accrualSysAddress, "api/orders"),
			Client: http.Client{},
		},
		AuthenticatedUser: User{
			Login:        "",
			HashPassword: "",
			ID:           1,
		},
		CheckOrderInterval: 5 * time.Second,
	}
}

func (db Database) SetStorage() error {
	driver, err := postgres.WithInstance(db.DB, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("could not create migration: %w", err)
	}

	if err != nil {
		log.Println("error while creating table:", err)
		return err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func (db Database) Finish() {
	db.DB.Close()
}
