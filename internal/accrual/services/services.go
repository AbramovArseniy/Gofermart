package services

import (
	"encoding/json"
	"fmt"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/luhnchecker"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"
)

func OrdersNumber(number string, GetOrderInfo func(number string) (types.OrdersInfo, error)) ([]byte, error) {
	var info types.OrdersInfo

	if luhnchecker.CalculateLuhn(number) {
		err := fmt.Errorf("wrong format of order number")
		info.Order = number
		info.Status = types.StatusInvalid
		i, error := json.Marshal(info)
		if error != nil {
			return nil, error
		}
		return i, err
	}

	info, err := GetOrderInfo(number)
	if err != nil {
		return nil, err
	}

	i, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	return i, nil
}
