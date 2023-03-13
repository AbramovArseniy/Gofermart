package services

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/luhnchecker"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/storage"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"
)

func OrderCheck(number string) ([]byte, int) {
	var info types.OrdersInfo

	if luhnchecker.CalculateLuhn(number) {
		info.Order = number
		info.Status = types.StatusInvalid
		i, error := json.Marshal(info)
		if error != nil {
			return nil, http.StatusInternalServerError
		}
		return i, http.StatusNoContent
	}
	return nil, http.StatusOK
}

func OrdersNumber(number string, keeper storage.Keeper) ([]byte, error) {
	var info types.OrdersInfo

	info, err := keeper.GetOrderInfo(number)
	if err != nil {
		return nil, err
	}

	i, err := json.Marshal(info)
	if err != nil {
		return nil, err
	}

	return i, nil
}

func OrderAdd(list io.ReadCloser, keeper storage.Keeper) error {
	var order types.CompleteOrder
	body, err := io.ReadAll(list)
	if err != nil {
		return err
	}

	log.Println(list)
	log.Println(string(body))

	json.Unmarshal(body, &order)

	log.Printf("order %+v", order)

	if keeper.FindOrder(order.Order) {
		err := fmt.Errorf("order already exist")
		return err
	}

	keeper.RegisterOrder(order)
	return nil
}
