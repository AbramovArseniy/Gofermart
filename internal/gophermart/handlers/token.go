package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strconv"
)

var superkey = []byte("fully protected encryption key")

func NewUserSign(id string) (string, error) {
	intID, err := strconv.Atoi(id)
	if err != nil {
		return "", fmt.Errorf("unable to convert Id (NewUserSign func): %w", err)
	}
	uint32ID := uint32(intID)
	databyte := binary.BigEndian.AppendUint32(nil, uint32ID)
	hash := hmac.New(sha256.New, superkey)
	hash.Write(databyte)
	sign := hash.Sum(nil)
	databyte = append(databyte, sign...)
	newsign := hex.EncodeToString(databyte)
	return newsign, nil
}

func GetUserSign(s string) (string, bool, error) {
	decodedbyte, err := hex.DecodeString(s)
	if err != nil {
		return "", false, err
	}
	id := binary.BigEndian.Uint32(decodedbyte[:4])
	userID := strconv.Itoa(int(id))
	hash := hmac.New(sha256.New, superkey)
	hash.Write(decodedbyte[:4])
	usersign := hash.Sum(nil)
	checkequal := hmac.Equal(usersign, decodedbyte[4:])
	return userID, checkequal, nil
}
