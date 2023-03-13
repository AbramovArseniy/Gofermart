package handlers

import (
	// "database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"

	// "math/rand"
	"net/http"
	// "time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/services"
	"github.com/labstack/echo/v4"
)

const (
	IntSymbols     = "0123456789"
	ShortURLMaxLen = 7
	userIDCookie   = "useridcookie"
)

// type Gophermart struct {
// 	Address            string
// 	Database           *sql.DB
// 	AccrualSysAddress  string
// 	Authenticated_user services.User
// 	Auth               services.Authorization
// 	// UserDatabase       *sql.DB
// }

func (g *Gophermart) Router() *echo.Echo {
	e := echo.New()
	// e.Use(Auth.AuthMiddleware())
	e.POST("/api/user/register", g.RegistHandler)
	e.POST("/api/user/login", g.AuthHandler)
	return e
}

func (g *Gophermart) RegistHandler(c echo.Context) error {
	var userData services.UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.RegisterUser(userData)
	if err != nil && !errors.Is(err, services.ErrInvalidData) {
		log.Printf("RegistHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "RegistHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, services.ErrInvalidData) {
		http.Error(c.Response().Writer, "RegistHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, err := g.Auth.GenerateToken(user)
	if err != nil {
		log.Printf("RegistHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "RegistHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

func (g *Gophermart) AuthHandler(c echo.Context) error {
	var userData services.UserData
	if err := json.NewDecoder(c.Request().Body).Decode(&userData); err != nil {
		http.Error(c.Response().Writer, err.Error(), http.StatusBadRequest)
		return nil
	}
	if err := userData.CheckData(); err != nil {
		http.Error(c.Response().Writer, fmt.Sprintf("no data provided: %s", err.Error()), http.StatusBadRequest)
		return nil
	}
	user, err := g.Auth.LoginUser(userData)
	if err != nil && !errors.Is(err, services.ErrInvalidData) {
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't login", http.StatusInternalServerError)
		return nil
	}
	if errors.Is(err, services.ErrInvalidData) {
		http.Error(c.Response().Writer, "AuthHandler: invalid login or password", http.StatusUnauthorized)
		return nil
	}
	token, err := g.Auth.GenerateToken(user)
	if err != nil {
		log.Printf("AuthHandler: error while register handler: %v", err)
		http.Error(c.Response().Writer, "AuthHandler: can't generate token", http.StatusInternalServerError)
		return nil
	}
	c.Response().Header().Set("Authorization", "Bearer "+token)
	c.Response().Writer.WriteHeader(http.StatusOK)
	return nil
}

// куки с моего проекта
// func UserIDfromCookie(r *http.Request) (string, *http.Cookie, error) {
// 	var cookie *http.Cookie
// 	sign, err := r.Cookie(userIDCookie)
// 	if err != nil {
// 		userID := GenerateRandomIntString()
// 		signValue, err := NewUserSign(userID)
// 		if err != nil {
// 			log.Println("Error of creating user sign (UserIDfromCookie)", err)
// 			return "", nil, err
// 		}
// 		cookie := &http.Cookie{Name: userIDCookie, Value: signValue, MaxAge: 0}
// 		return userID, cookie, nil
// 	}
// 	signValue := sign.Value
// 	userID, checkAuth, err := GetUserSign(signValue)
// 	if err != nil {
// 		log.Println("Error when getting of user sign (UserIDfromCookie)", err)
// 		return "", nil, err
// 	}
// 	if !checkAuth {
// 		log.Println("Error of equal checking (UserIDfromCookie)", err)
// 		return "", nil, err
// 	}
// 	return userID, cookie, nil
// }

// func GenerateRandomIntString() string {
// 	rand.Seed(time.Now().UnixNano())
// 	result := make([]byte, 0, ShortURLMaxLen)
// 	for i := 0; i < ShortURLMaxLen; i++ {
// 		s := IntSymbols[rand.Intn(len(IntSymbols))]
// 		result = append(result, s)
// 	}
// 	return string(result)
// }
