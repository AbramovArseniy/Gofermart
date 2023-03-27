package types

type status string

type OrdersInfo struct {
	Order   string  `json:"order"`
	Status  status  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

type (
	CompleteOrder struct {
		Order string        `json:"order"`
		Goods []ordersGoods `json:"goods"`
	}

	ordersGoods struct {
		Description string  `json:"description"`
		Price       float64 `json:"price"`
	}
)

type Goods struct {
	Match      string  `json:"match"`
	Reward     float64 `json:"reward"`
	RewardType string  `json:"reward_type"`
}

const (
	StatusRegistred  status = "REGISTERED"
	StatusInvalid    status = "INVALID"
	StatusProcessing status = "PROCESSING"
	StatusProcesed   status = "PROCESSED"
)
