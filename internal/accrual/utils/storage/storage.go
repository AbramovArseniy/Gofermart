package storage

import (
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"
)

type Keeper interface {
	GetOrderInfo(number string) (types.OrdersInfo, error)
	CheckOrderStatus(number string) bool
	CheckGoods(match string) bool
	RegisterOrder(types.CompleteOrder) error
	RegisterGoods(types.Goods) error
	UpdateOrderStatus(types.OrdersInfo) error
	FindOrder(number string) bool
	FindGoods(order types.CompleteOrder) (float64, error)
}
