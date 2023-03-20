package handlers

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/database"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
	"github.com/go-chi/jwtauth"
	jwx "github.com/lestrrat-go/jwx/jwt"
	"golang.org/x/crypto/bcrypt"
)

const (
	UserIDReq    = "user_id"
	UserLoginReq = "login"
)

type AuthJWT struct {
	UserStorage types.UserDB
	AuthToken   *jwtauth.JWTAuth
	context     context.Context
}

func NewAuth(store types.UserDB, secret string, context context.Context) *AuthJWT {
	jwtAuth := jwtauth.New("HS256", []byte(secret), nil)

	return &AuthJWT{
		AuthToken:   jwtAuth,
		UserStorage: store,
		context:     context,
	}
}

func (a *AuthJWT) CheckData(u types.UserData) error {
	if u.Login == "" {
		return errors.New("error: login is empty")
	}
	if u.Password == "" {
		return errors.New("error: password is empty")
	}

	return nil
}

func (a *AuthJWT) RegisterUser(userdata types.UserData) (types.User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(userdata.Password), bcrypt.DefaultCost)
	if err != nil {
		log.Println("error during bcrypt")
		return types.User{}, types.ErrHashGenerate
	}
	user, err := a.UserStorage.RegisterNewUser(userdata.Login, string(hash))
	if err != nil {
		log.Println("error Register New User")
		return types.User{}, database.ErrUserExists
	}

	return user, nil
}

func (a *AuthJWT) LoginUser(userdata types.UserData) (types.User, error) {
	user, err := a.UserStorage.GetUserData(userdata.Login)
	if err != nil {
		return types.User{}, err
	}
	if user.ID == 0 {
		return types.User{}, types.ErrInvalidData
	}
	if err = bcrypt.CompareHashAndPassword([]byte(user.HashPassword), []byte(userdata.Password)); err != nil {
		return types.User{}, types.ErrInvalidData
	}

	return user, nil
}

func (a *AuthJWT) GenerateToken(user types.User) (string, error) {

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

func (a *AuthJWT) getTokenReqs(user types.User) (map[string]interface{}, error) {
	reqs := map[string]interface{}{}
	jwtauth.SetIssuedNow(reqs)
	duration, err := time.ParseDuration("10h")
	if err != nil {
		return nil, err
	}
	jwtauth.SetExpiryIn(reqs, duration)
	if user.Login == "" {
		return nil, errors.New("user login is required")
	}
	reqs[UserIDReq] = user.ID
	reqs[UserLoginReq] = user.Login

	return reqs, nil
}

func (a *AuthJWT) GetUserID(r *http.Request) int {
	token := a.verify(r, jwtauth.TokenFromCookie, jwtauth.TokenFromHeader)

	var err error
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

func (a *AuthJWT) GetUserLogin(r *http.Request) string {
	token := a.verify(r, jwtauth.TokenFromCookie, jwtauth.TokenFromHeader)

	var login string

	buserID, exist := token.Get(UserLoginReq)
	if exist {
		login, _ = buserID.(string)
	}

	return login
}

func (a *AuthJWT) verify(r *http.Request, findTokenFns ...func(r *http.Request) string) jwx.Token {
	token, err := jwtauth.VerifyRequest(a.AuthToken, r, findTokenFns...)
	if err != nil {
		log.Println("ошибка в verify", err)
	}

	return token
}
