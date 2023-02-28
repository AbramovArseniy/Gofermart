package storage

import (
	"errors"
)

type Maininterface interface {
	AddData(login, password string) error
	CheckLoginUnicality(login string) (error, bool)
}

var (
	ErrDataExists = errors.New("such URl already exist in DB")
)
