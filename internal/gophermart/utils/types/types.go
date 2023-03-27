package types

import (
	"errors"
	"net/http"
	"net/url"
	"time"
)

type Authorization interface {
	GenerateToken(user User) (string, error)
	RegisterUser(userdata UserData) (User, error)
	LoginUser(userdata UserData) (User, error)
	GetUserID(r *http.Request) int
	GetUserLogin(r *http.Request) string
	CheckData(u UserData) error
}

type Storage interface {
	SaveOrder(order *Order) error
	SaveWithdrawal(withdrawal Withdrawal, authUserLogin string) error
	GetOrderUserByNum(orderNum string) (user string, exists bool, err error)
	GetOrderUser(orderNum string) (userID string, err error)
	GetOrdersByUser(authUserID string) (orders []Order, exist bool, err error)
	GetBalance(authUserLogin string) (balance float64, withdrawn float64, err error)
	UpgradeOrderStatus(body []byte, orderNum string) error
	GetWithdrawalsByUser(authUserLogin string) (withdrawals []Withdrawal, exists bool, err error)
	CheckOrders(accrualSysClient Client)
	CheckUserData(login, hash string) bool
	RegisterNewUser(login string, password string) (User, error)
	GetUserData(login string) (User, error)
	Close()
}

type UserDB interface {
	RegisterNewUser(login string, password string) (User, error)
	GetUserData(login string) (User, error)
}

type User struct {
	Login        string
	HashPassword string
	ID           int
}
type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type Withdrawal struct {
	UserID      int
	OrderNum    string    `json:"order"`
	Accrual     float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

type Client struct {
	URL    url.URL
	Client http.Client
}

type Balance struct {
	Balance   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
type Order struct {
	User       string
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    float64   `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

var (
	ErrUserExists   = errors.New("such user already exist in DB")
	ErrScanData     = errors.New("error while scan user ID")
	ErrInvalidData  = errors.New("error user data is invalid")
	ErrHashGenerate = errors.New("error can't generate hash")
	ErrKeyNotFound  = errors.New("error user ID not found")
	ErrAlarm        = errors.New("error tx.BeginTx alarm")
	ErrAlarm2       = errors.New("error tx.PrepareContext alarm")
)
