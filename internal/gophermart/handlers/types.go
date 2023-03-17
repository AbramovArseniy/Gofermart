package handlers

import (
	"net/http"
	"path"
	"time"
)

type Storage interface {
	SaveOrder(order *Order) error
	SaveWithdrawal(withdrawal Withdrawal, authUserID int) error
	GetOrderUserByNum(orderNum string) (userID int, exists bool, err error)
	GetOrdersByUser(authUserID int) (orders []Order, exist bool, err error)
	GetBalance(authUserID int) (balance float64, withdrawn float64, err error)
	// GetUserData(login string) (User, error)
	// RegisterNewUser(login string, password string) (User, error)
	UpgradeOrderStatus(accrualSysClient Client, orderNum string) error
	GetWithdrawalsByUser(authUserID int) (withdrawals []Withdrawal, exists bool, err error)
	CheckOrders(accrualSysClient Client)
	CheckUserData(login, hash string) bool
	Close()
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

type Gophermart struct {
	Storage            Storage
	AccrualSysClient   Client
	AuthenticatedUser  User
	CheckOrderInterval time.Duration
	Auth               Authorization // added A
	secret             string
}

func NewGophermart(accrualSysAddress, secret string, database *DataBase, auth *AuthJWT) *Gophermart {
	return &Gophermart{
		Storage: database,
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
		Auth:               auth,
		secret:             secret,
	}
}
