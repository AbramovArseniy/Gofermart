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
	var orderInfo types.OrdersInfo

	if !luhnchecker.CalculateLuhn(number) {
		orderInfo.Order = number
		orderInfo.Status = types.StatusInvalid
		i, err := json.Marshal(orderInfo)
		if err != nil {
			return nil, http.StatusInternalServerError
		}
		return i, http.StatusOK
	}
	return nil, http.StatusOK
}

func OrdersNumber(number string, keeper storage.Keeper) ([]byte, error) {
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

func OrderAdd(list io.Reader, keeper storage.Keeper) (int, error) {
	var (
		order     types.CompleteOrder
		orderInfo types.OrdersInfo
		accrual   float64
	)

	body, err := io.ReadAll(list)
	if err != nil {
		return http.StatusBadRequest, err
	}

	err = json.Unmarshal(body, &order)
	if err != nil {
		log.Println("error unm")
		return http.StatusBadRequest, err
	}

	if keeper.CheckOrderStatus(order.Order) {
		err := fmt.Errorf("order already processing")
		return http.StatusConflict, err
	}

	orderInfo.Order = order.Order

	err = keeper.RegisterOrder(order)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	orderInfo.Status = types.StatusProcessing

	err = keeper.UpdateOrderStatus(orderInfo)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	accrual, err = keeper.FindGoods(order)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	orderInfo.Status = types.StatusProcesed
	orderInfo.Accrual = accrual

	err = keeper.UpdateOrderStatus(orderInfo)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusAccepted, nil
}

func GoodsAdd(newGoods io.Reader, keeper storage.Keeper) (int, error) {
	var goods types.Goods

	body, err := io.ReadAll(newGoods)
	if err != nil {
		return http.StatusBadRequest, err
	}

	err = json.Unmarshal(body, &goods)
	if err != nil {
		log.Println("error unm")
		return http.StatusBadRequest, err
	}

	if keeper.CheckGoods(goods.Match) {
		err := fmt.Errorf("goods already registred")
		return http.StatusConflict, err
	}

	err = keeper.RegisterGoods(goods)
	if err != nil {
		return http.StatusInternalServerError, err
	}

	return http.StatusOK, nil
}
