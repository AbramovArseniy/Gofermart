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
	GetOrderUserByNum(orderNum string) (userID int, exists bool, err error)
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
	OrderNum    string    `json:"order"`
	Accrual     float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type Database struct {
	Storage
	DB                            *sql.DB
	CheckOrderInterval            time.Duration
	SelectBalacneAndWithdrawnStmt *sql.Stmt
	InsertWirdrawal               *sql.Stmt
	SelectWithdrawalsByUser       *sql.Stmt
}

func NewDatabase(db *sql.DB) Database {
	selectBalacneAndWithdrawnStmt, err := db.Prepare(`SELECT (orders.accrual_sum - withdrawals.withdrawal_sum, withdrawals.withdrawal_sum)
	FROM (SELECT SUM(accrual) AS accrual_sum FROM orders WHERE status = PROCESSED AND user_id = $1) orders,
	(SELECT SUM(accrual) AS withdrawal_sum FROM withdrawals) withdrawals WHERE user_id = $1`)
	if err != nil {
		log.Println("cannot prepare selectBalacneAndWithdrawnStmt:", err)
	}
	insertWirdrawal, err := db.Prepare("INSERT (user_id, order_num, accrual, created_at) INTO withdrawals VALUES ($1, $2, $3, $4)")
	if err != nil {
		log.Println("cannot prepare insertWirdrawal:", err)
	}
	selectWithdrawalsByUser, err := db.Prepare(`SELECT (order_num, accrual, created_at) FROM withdrawals WHERE user_id=$1`)
	if err != nil {
		log.Println("cannot prepare selectWithdrawalsByUser:", err)
	}
	return Database{
		DB:                            db,
		SelectBalacneAndWithdrawnStmt: selectBalacneAndWithdrawnStmt,
		InsertWirdrawal:               insertWirdrawal,
		SelectWithdrawalsByUser:       selectWithdrawalsByUser,
	}
}

type Balance struct {
	Balance   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
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
