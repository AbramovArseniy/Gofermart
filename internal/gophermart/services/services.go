package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"path"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/luhnchecker"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
)

func RegistService(r *http.Request, auth types.Authorization) (int, string, error) {
	var (
		userData types.UserData
		token    string
	)
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {

		return http.StatusBadRequest, token, nil
	}
	if err := auth.CheckData(userData); err != nil {
		return http.StatusBadRequest, token, fmt.Errorf("no data provided: %s", err.Error())
	}
	user, err := auth.RegisterUser(userData)
	if err != nil && !errors.Is(err, types.ErrInvalidData) {
		return http.StatusLoopDetected, token, fmt.Errorf("RegistHandler: %s", err.Error())
	}
	if errors.Is(err, types.ErrInvalidData) {

		return http.StatusUnauthorized, token, fmt.Errorf("RegistHandler: %s", err.Error())
	}
	token, err = auth.GenerateToken(user)
	if err != nil {
		return http.StatusInternalServerError, token, fmt.Errorf("RegistHandler: can't generate token %s", err.Error())
	}

	return http.StatusOK, token, nil
}

func AuthService(r *http.Request, storage types.Storage, auth types.Authorization) (int, string, error) {
	var (
		userData types.UserData
		token    string
	)
	if err := json.NewDecoder(r.Body).Decode(&userData); err != nil {
		return http.StatusBadRequest, token, err
	}
	if err := auth.CheckData(userData); err != nil {
		return http.StatusBadRequest, token, err
	}
	user, err := auth.LoginUser(userData)
	if err != nil && !errors.Is(err, types.ErrInvalidData) {
		return http.StatusInternalServerError, token, err
	}
	if errors.Is(err, types.ErrInvalidData) {
		return http.StatusUnauthorized, token, err
	}
	token, err = auth.GenerateToken(user)
	if err != nil {
		return http.StatusInternalServerError, token, err
	}

	return http.StatusOK, token, err
}

func PostOrderService(r *http.Request, storage types.Storage, auth types.Authorization, accrualSysClient types.Client) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err := fmt.Errorf("cannot read request body %s", err)
		return http.StatusInternalServerError, err
	}
	orderNum := string(body)
	log.Printf(`PostOrderService: orderNum: %s`, orderNum)
	numIsRight := luhnchecker.OrderNumIsRight(orderNum)
	if !numIsRight {
		err := fmt.Errorf("luhnchecker %t", numIsRight)
		return http.StatusUnprocessableEntity, err
	}
	userid := auth.GetUserLogin(r)
	log.Printf("GetOrderService: USER LOGIN: %s", userid)

	oldOrder, err := storage.FindOldOrder(orderNum)
	if err != nil {
		return 0, err
	}
	if oldOrder.Number != "0" {
		return 0, err
	}

	newOrder, err := storage.AddNewOrder(orderNum, userid)
	if err != nil {
		return 0, err
	}

	// user, exists, err := storage.GetOrderUserByNum(orderNum)
	// if err != nil {
	// 	err := fmt.Errorf("cannot get user id by order number %s", err)
	// 	return http.StatusInternalServerError, err
	// }
	// log.Printf(`PostOrderService: user: %s`, user)

	order := types.Order{
		User:   auth.GetUserLogin(r),
		Number: orderNum,
		Status: "NEW",
	}
	log.Printf(`PostOrderService: order: %v`, order)
	if !exists {
		err = storage.SaveOrder(&order)
		if err != nil {
			err := fmt.Errorf("cannot save order %s", err)
			return http.StatusInternalServerError, err
		}
		url := accrualSysClient.URL
		url.Path = path.Join(accrualSysClient.URL.Path, orderNum)
		resp, err := accrualSysClient.Client.Get(url.String())
		if err != nil {
			log.Println("can't get response from accrual sytem:", err)
			return http.StatusInternalServerError, err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Println("can't read body of reponse from accrual sytem:", err)
			return http.StatusInternalServerError, err
		}
		if resp.StatusCode > 299 {
			log.Println("accrual system returned statuscode:", resp.StatusCode)
			return http.StatusInternalServerError, fmt.Errorf("accrual system returned statuscode: %d", resp.StatusCode)
		}
		storage.UpgradeOrderStatus(body, orderNum)
		return http.StatusAccepted, err

	}
	log.Printf("id in order %s, id in req %s", user, auth.GetUserLogin(r))
	if user != auth.GetUserLogin(r) {
		err = fmt.Errorf("order already uploaded by another user")

		return http.StatusConflict, err
	}
	return http.StatusOK, fmt.Errorf("order already uploaded by you")
}

// func GetOrderService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
func GetOrderService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []types.OrderResp, error) {
	userid := auth.GetUserLogin(r)
	log.Printf("GetOrderService: USER LOGIN: %s", userid)
	orders, exist, err := storage.GetOrdersByUser(userid)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("GetOrdersHandler: error while getting orders by user: %w", err)
	}
	log.Printf("GetOrderService: ORDERS: %v", orders)
	if !exist {
		err = fmt.Errorf("order exists? %t", exist)
		return http.StatusNoContent, nil, err
	}
	//albert
	res := make([]types.OrderResp, len(orders))

	for i, order := range orders {
		res[i] = types.OrderResp{
			Number:     order.Number,
			Status:     order.Status,
			Accrual:    math.Round(order.Accrual*100) / 100,
			UploadedAt: order.UploadedAt.Format(time.RFC3339),
		}
	}
	log.Printf("GetOrderService: RESULT ORDERS: %v", res)
	// var body []byte
	// if body, err = json.Marshal(&orders); err != nil {
	// 	return http.StatusInternalServerError, nil, err
	// }
	// log.Println("GetOrderService: EVERYTHING still is OK #4")
	// return http.StatusOK, body, nil
	return http.StatusOK, res, nil
}

func PostWithdrawalService(r *http.Request, storage types.Storage, auth types.Authorization) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {

		return http.StatusInternalServerError, fmt.Errorf("error while reading request body: %w", err)
	}
	var w types.Withdrawal
	err = json.Unmarshal(body, &w)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error while reading request body: %w", err)
	}
	if !luhnchecker.OrderNumIsRight(w.OrderNum) {
		return http.StatusUnprocessableEntity, fmt.Errorf("wrong order number ")
	}
	balance, _, err := storage.GetBalance(auth.GetUserLogin(r))
	if err != nil {

		return http.StatusInternalServerError, fmt.Errorf("error while counting balance: %w", err)
	}
	if balance < w.Accrual {
		return http.StatusPaymentRequired, fmt.Errorf("not enough accrual on balance")
	}
	storage.SaveWithdrawal(w, auth.GetUserLogin(r))

	return http.StatusOK, nil
}

func GetBalanceService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
	var (
		b        types.Balance
		response []byte
		err      error
	)
	b.Balance, b.Withdrawn, err = storage.GetBalance(auth.GetUserLogin(r))
	if err != nil {

		return http.StatusInternalServerError, response, fmt.Errorf("GetBalanceService: error while counting balance: %w", err)
	}
	response, err = json.Marshal(b)
	if err != nil {
		return http.StatusInternalServerError, response, fmt.Errorf("GetBalanceService: error while marshaling response json: %w", err)
	}

	return http.StatusOK, response, nil
}

func GetWithdrawalsService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
	var response []byte
	w, exist, err := storage.GetWithdrawalsByUser(auth.GetUserLogin(r))
	if err != nil {
		return http.StatusInternalServerError, response, fmt.Errorf("error while getting user's withdrawals: %w", err)
	}
	if !exist {

		return http.StatusNoContent, response, fmt.Errorf("is withdrawal exist? %t", exist)
	}
	response, err = json.Marshal(w)
	if err != nil {
		return http.StatusInternalServerError, response, fmt.Errorf("error while marshaling response json: %w", err)
	}
	return http.StatusOK, response, nil
}
