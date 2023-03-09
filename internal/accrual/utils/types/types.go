package types

type status string

type OrdersInfo struct {
	Order   string `json:"order"`
	Status  status `json:"status"`
	Accrual int64  `json:"accrual"`
}

type (
	CompleteOrder struct {
		Order string        `json:"order"`
		Goods []ordersGoods `json:"goods"`
	}

	ordersGoods struct {
		Description string `json:"descriptioin"`
		Price       int64  `json:"price"`
	}
)

type Goods struct {
	Match       string `json:"match"`
	Reward      int    `json:"reward"`
	Reward_type string `json:"rewardtype"`
}

const (
	StatusRegistred  status = "REGISTERED"
	StatusInvalid    status = "INVALID"
	StatusProcessing status = "PROCESSING"
	StatusProcesed   status = "PROCESSED"
)
