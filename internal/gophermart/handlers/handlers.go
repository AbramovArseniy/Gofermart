package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
)

type LoginPassword struct {
	Login    string `json:"login,omitempty"`
	Password string `json:"password,omitempty"`
}

func RegistHandler(w http.ResponseWriter, r *http.Request) {
	var dataStrorage LoginPassword
	if err := json.NewDecoder(r.Body).Decode(&dataStrorage); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	login, err := url.QueryUnescape(dataStrorage.Login)
	if err != nil {
		http.Error(w, "RegistHandler: unable to QueryUnescape login", http.StatusBadRequest)
		return
	}
	password, err := url.QueryUnescape(dataStrorage.Password)
	if err != nil {
		http.Error(w, "RegistHandler: unable to QueryUnescape password", http.StatusBadRequest)
		return
	}
	AddData
	err, ok := CheckLoginUnicality(login)
	if !ok {
		log.Println("RegistHandler: login is already taken")
		w.WriteHeader(http.StatusConflict)
		return
	}
}
