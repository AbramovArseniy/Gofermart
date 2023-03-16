package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/jwtauth"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserExists      = errors.New("such user already exist in DB")
	ErrNewRegistration = errors.New("error while register user")
	ErrInvalidData     = errors.New("error user data is invalid")
	ErrHashGenerate    = errors.New("error can't generate hash")
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

// type UserDB interface {
// 	RegisterNewUser(login string, password string) (User, error)
// 	GetUserData(login string) (User, error)
// }

const UserIDReq = "user_id"

type Authorization interface {
	GenerateToken(user User) (string, error)
	GetUserID(r *http.Request) int
	AuthMiddleware() func(h http.Handler) http.Handler
}

type AuthJWT struct {
	AuthToken *jwtauth.JWTAuth
}

func NewAuth(secret string) *AuthJWT {
	jwtAuth := jwtauth.New("HS256", []byte(secret), nil)
	return &AuthJWT{
		AuthToken: jwtAuth,
	}
}

func (g *Gophermart) RegisterUser(userdata UserData) (User, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(userdata.Password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, ErrHashGenerate
	}
	user, err := g.Storage.RegisterNewUser(userdata.Login, string(hash))
	if err != nil && !errors.Is(err, ErrUserExists) {
		return User{}, ErrNewRegistration
	}
	if errors.Is(err, ErrUserExists) {
		return User{}, ErrUserExists
	}
	return user, nil
}

func (g *Gophermart) LoginUser(userdata UserData) (User, error) {
	user, err := g.Storage.GetUserData(userdata.Login)
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
	return reqs, nil
}

func (a *AuthJWT) GetUserID(r *http.Request) int {
	_, reqs, _ := jwtauth.FromContext(r.Context())
	userID := reqs[UserIDReq].(float64)
	return int(userID)
}

func (a *AuthJWT) AuthMiddleware() func(h http.Handler) http.Handler {
	verifier := jwtauth.Verifier(a.AuthToken)
	authenticator := jwtauth.Authenticator
	return func(h http.Handler) http.Handler {
		return verifier(authenticator(h))
	}
}
