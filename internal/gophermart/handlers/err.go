package handlers

import (
	"fmt"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
)

type ErrUnique struct {
	Err   error
	Field string
}

func NewErrUnique(field string, err error) error {
	return &ErrUnique{
		Err:   err,
		Field: field,
	}
}

func (err *ErrUnique) Error() string {
	return fmt.Sprintf("not unique data: %s", err.Field)
}

func (err *ErrUnique) Unwrap() error {
	return err.Err
}

type ErrorRegist struct {
	Err  error
	Data types.UserData
}

func NewErrorRegist(userdata types.UserData, err error) error {
	return &ErrorRegist{
		Err:  err,
		Data: userdata,
	}
}

func (err *ErrorRegist) Error() string {
	return fmt.Sprintf("problem while register user with this userdata: %+v\n%s", err.Data, err.Err.Error())
}

func (err *ErrorRegist) Unwrap() error {
	return err.Err
}
