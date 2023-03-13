package storage

import "github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"

type Keeper interface {
	GetOrderInfo(number string) (types.OrdersInfo, error)
	FindOrder(number string) bool
	RegisterOrder(types.CompleteOrder) error
}
