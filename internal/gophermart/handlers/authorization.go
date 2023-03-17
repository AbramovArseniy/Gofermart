package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/jwtauth"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists = errors.New("such user already exist in DB")
	// ErrNewRegistration = errors.New("error while register user - main problem")
	ErrScanData     = errors.New("error while scan user ID")
	ErrInvalidData  = errors.New("error user data is invalid")
	ErrHashGenerate = errors.New("error can't generate hash")
	ErrKeyNotFound  = errors.New("error user ID not found")
	ErrAlarm        = errors.New("error tx.BeginTx alarm")
	ErrAlarm2       = errors.New("error tx.PrepareContext alarm")
)

type User struct {
	Login        string
	HashPassword string
	ID           int
}

type UserData struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

func (u UserData) CheckData() error {
	if u.Login == "" {
		return errors.New("error: login is empty")
	}
	if u.Password == "" {
		return errors.New("error: password is empty")
	}
	return nil
}

type UserDB interface {
	RegisterNewUser(login string, password string) (User, error)
	GetUserData(login string) (User, error)
}

const UserIDReq = "user_id"

type Authorization interface {
	GenerateToken(user User) (string, error)
	RegisterUser(userdata UserData) (User, error)
	LoginUser(userdata UserData) (User, error)
	GetUserID(h http.Header) int
}

type AuthJWT struct {
	UserStorage UserDB
	AuthToken   *jwtauth.JWTAuth
	context     context.Context
}

func NewAuth(store UserDB, secret string, context context.Context) *AuthJWT {
	jwtAuth := jwtauth.New("HS256", []byte(secret), nil)
	return &AuthJWT{
		AuthToken:   jwtAuth,
		UserStorage: store,
		context:     context,
	}
}

func (a *AuthJWT) RegisterUser(userdata UserData) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(userdata.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, ErrHashGenerate
	}
	user, err := a.UserStorage.RegisterNewUser(userdata.Login, string(hash))
	var ErrUnique *ErrUnique
	if errors.As(err, &ErrUnique) {
		return User{}, ErrUserExists
	}
	if err != nil {
		return User{}, NewErrorRegist(userdata, err)
	}
	// if err != nil && !errors.Is(err, ErrUserExists) {
	// 	return User{}, ErrNewRegistration
	// }
	// if errors.Is(err, ErrUserExists) {
	// 	return User{}, ErrUserExists
	// }
	return user, nil
}

func (a *AuthJWT) LoginUser(userdata UserData) (User, error) {
	user, err := a.UserStorage.GetUserData(userdata.Login)
	if err != nil {
		return User{}, err
	}
	if user.ID == 0 {
		return User{}, ErrInvalidData
	}
	if err = bcrypt.CompareHashAndPassword([]byte(user.HashPassword), []byte(userdata.Password)); err != nil {
		return User{}, ErrInvalidData
	}
	return user, nil
}

func (a *AuthJWT) GenerateToken(user User) (string, error) {
	reqs, err := a.getTokenReqs(user)
	if err != nil {
		return "", err
	}
	_, tokenString, err := a.AuthToken.Encode(reqs)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (a *AuthJWT) getTokenReqs(user User) (map[string]interface{}, error) {
	reqs := map[string]interface{}{}
	jwtauth.SetIssuedNow(reqs)
	duration, err := time.ParseDuration("10h")
	if err != nil {
		return nil, err
	}
	jwtauth.SetExpiryIn(reqs, duration)
	if user.ID == 0 {
		return nil, errors.New("user id is required")
	}
	reqs[UserIDReq] = user.ID
	log.Println(user.ID)
	return reqs, nil
}

func (a *AuthJWT) GetUserID(header http.Header) int {
	tokenString := TokenFromHeader(header)
	token, err := a.AuthToken.Decode(tokenString)
	if err != nil {
		log.Println(err)
	}

	var claims map[string]interface{}

	if token != nil {
		claims, err = token.AsMap(context.Background())
		if err != nil {
			log.Println(err)
		}
	} else {
		claims = map[string]interface{}{}
	}
	userID, _ := claims[UserIDReq].(float64)
	return int(userID)
}

func TokenFromHeader(header http.Header) string {
	bearer := header.Get("Authorization")
	if len(bearer) > 7 && strings.ToUpper(bearer[0:6]) == "BEARER" {
		return bearer[7:]
	}
	return ""
}
