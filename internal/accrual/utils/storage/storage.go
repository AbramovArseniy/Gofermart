package storage

import "github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"

type Keeper interface {
	RegisterNewOrder() error
	DBRegRequst() error
	DBGetOrderAccrual() error
	DBSetNewGoods() error
	GetOrderInfo(number string) (types.OrdersInfo, error)
}

type Storage struct {
	Keeper Keeper
}

func New(keeper Keeper) *Storage {
	return &Storage{
		Keeper: keeper,
	}
}
