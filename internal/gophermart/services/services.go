package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"

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

func PostOrderService(r *http.Request, storage types.Storage, auth types.Authorization) (int, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		err := fmt.Errorf("cannot read request body %s", err)
		return http.StatusInternalServerError, err
	}

	orderNum := string(body)
	numIsRight := luhnchecker.OrderNumIsRight(orderNum)
	user, exists, err := storage.GetOrderUserByNum(orderNum)
	if err != nil {
		err := fmt.Errorf("cannot get user id by order number %s", err)
		return http.StatusInternalServerError, err
	}

	if !numIsRight {
		err := fmt.Errorf("luhnchecker %t", numIsRight)
		return http.StatusUnprocessableEntity, err
	}
	order := types.Order{
		User:   auth.GetUserLogin(r),
		Number: orderNum,
		Status: "NEW",
	}
	if !exists {
		err = storage.SaveOrder(&order)
		if err != nil {
			err := fmt.Errorf("cannot save order %s", err)
			return http.StatusInternalServerError, err
		}
		return http.StatusAccepted, err
	}
	log.Printf("id in order %s, id in req %s", user, auth.GetUserLogin(r))
	if user != auth.GetUserLogin(r) {
		err = fmt.Errorf("order already uploaded by another user")

		return http.StatusConflict, err
	}
	return http.StatusOK, fmt.Errorf("order already uploaded by you")
}

func GetOrderService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
	userid := auth.GetUserID(r)
	orders, exist, err := storage.GetOrdersByUser(userid)
	if err != nil {
		return http.StatusInternalServerError, nil, fmt.Errorf("GetOrdersHandler: error while getting orders by user: %w", err)
	}
	if !exist {
		err = fmt.Errorf("order exists? %t", exist)
		return http.StatusNoContent, nil, err
	}
	var body []byte
	if body, err = json.Marshal(&orders); err != nil {
		return http.StatusInternalServerError, nil, err
	}

	return http.StatusOK, body, nil
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
	balance, _, err := storage.GetBalance(auth.GetUserID(r))
	if err != nil {

		return http.StatusInternalServerError, fmt.Errorf("error while counting balance: %w", err)
	}
	if balance < w.Accrual {
		return http.StatusPaymentRequired, fmt.Errorf("not enough accrual on balance")
	}
	storage.SaveWithdrawal(w, auth.GetUserID(r))

	return http.StatusOK, nil
}

func GetBalanceService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
	var (
		b        types.Balance
		response []byte
		err      error
	)
	b.Balance, b.Withdrawn, err = storage.GetBalance(auth.GetUserID(r))
	if err != nil {

		return http.StatusInternalServerError, response, fmt.Errorf("error while counting balance: %w", err)
	}
	response, err = json.Marshal(b)
	if err != nil {
		return http.StatusInternalServerError, response, fmt.Errorf("error while marshling response json: %w", err)
	}

	return http.StatusOK, response, nil
}

func GetWithdrawalsService(r *http.Request, storage types.Storage, auth types.Authorization) (int, []byte, error) {
	var response []byte
	w, exist, err := storage.GetWithdrawalsByUser(auth.GetUserID(r))
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
