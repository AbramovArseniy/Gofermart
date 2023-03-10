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
	OrderNum    string
	Accrual     float64
	ProcessedAt time.Time
}

type Database struct {
	Storage
	DB                 *sql.DB
	CheckOrderInterval time.Duration
}

func NewDatabase(DBURI string) {

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
		Storage: Database{
			DB:                 db,
			CheckOrderInterval: 5 * time.Second,
		},
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
