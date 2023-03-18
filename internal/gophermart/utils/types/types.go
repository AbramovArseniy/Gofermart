package types

import (
	"errors"
	"net/http"
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
	SaveWithdrawal(withdrawal Withdrawal, authUserID int) error
	GetOrderUserByNum(orderNum string) (userID int, exists bool, err error)
	GetOrdersByUser(authUserID int) (orders []Order, exist bool, err error)
	GetBalance(authUserID int) (balance float64, withdrawn float64, err error)
	// GetUserData(login string) (User, error)
	// RegisterNewUser(login string, password string) (User, error)
	UpgradeOrderStatus(body []byte, orderNum string) error
	GetWithdrawalsByUser(authUserID int) (withdrawals []Withdrawal, exists bool, err error)
	CheckOrders(body []byte)
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
	URL    string
	Client http.Client
}

type Balance struct {
	Balance   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}
type Order struct {
	UserID     int       `json:"-"`
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

var (
	ErrUserExists = errors.New("such user already exist in DB")
	// ErrNewRegistration = errors.New("error while register user - main problem")
	ErrScanData     = errors.New("error while scan user ID")
	ErrInvalidData  = errors.New("error user data is invalid")
	ErrHashGenerate = errors.New("error can't generate hash")
	ErrKeyNotFound  = errors.New("error user ID not found")
	ErrAlarm        = errors.New("error tx.BeginTx alarm")
	ErrAlarm2       = errors.New("error tx.PrepareContext alarm")
)