package storage

import "time"

type User struct {
	ID int64
}

type Order struct {
	UserID     int       `json:"user_id,omitempty"`
	Number     int64     `json:"number"`
	Status     string    `json:"status"`
	Accrual    int       `json:"accrual,omitempty"`
	UploadedAt time.Time `json:"uploaded_at"`
}

type Storage struct {
	Users  []User
	Orders []Order
}

func (s *Storage) GetOrdersByUser(UserID int) (orders []Order) {
	for _, order := range s.Orders {
		if order.UserID == UserID {
			orders = append(orders, order)
		}
	}
	return
}

func (s *Storage) GetOrderByNumber(orderNum int64) *Order {
	for _, order := range s.Orders {
		if order.Number == orderNum {
			return &order
		}
	}
	return nil
}

func (s *Storage) AppendOrder(order *Order) {
	s.Orders = append(s.Orders, *order)
}
