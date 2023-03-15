package types

type status string

type OrdersInfo struct {
	Order   string `json:"order"`
	Status  status `json:"status"`
	Accrual int    `json:"accrual,omitempty"`
}

type (
	CompleteOrder struct {
		Order string        `json:"order"`
		Goods []ordersGoods `json:"goods"`
	}

	ordersGoods struct {
		Description string  `json:"description"`
		Price       float32 `json:"price"`
	}
)

type Goods struct {
	Match      string `json:"match"`
	Reward     int    `json:"reward"`
	RewardType string `json:"rewardtype"`
}

const (
	StatusRegistred  status = "REGISTERED"
	StatusInvalid    status = "INVALID"
	StatusProcessing status = "PROCESSING"
	StatusProcesed   status = "PROCESSED"
)
