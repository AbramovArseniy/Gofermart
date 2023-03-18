package main

import (
	"fmt"
	"log"
	rnd "math/rand"
	"net/http/cookiejar"
	"strconv"

	"github.com/go-resty/resty/v2"
)

func main() {
	jar, _ := cookiejar.New(nil)

	log.Println(jar)
	httpc := resty.New().SetBaseURL("http://127.0.0.1:8080").SetCookieJar(jar)

	login := ASCIIString(7, 14)
	password := ASCIIString(16, 32)

	orderNum, err := generateOrderNumber()
	if err != nil {
		log.Println(err)
	}
	log.Println(orderNum)
	m := []byte(`{"login": "` + login + `","password": "` + password + `"}`)

	req := httpc.R().SetHeader("Content-Type", "application/json").SetBody(m)

	resp, err := req.Post("/api/user/register")
	log.Println(resp.StatusCode(), err)

	body := []byte(orderNum)

	req = httpc.R().SetHeader("Content-Type", "text/plain").SetBody(body)

	resp, err = req.Post("/api/user/orders")
	log.Println(resp.StatusCode(), err)

	body = []byte(orderNum)

	req = httpc.R().SetHeader("Content-Type", "text/plain").SetBody(body)

	resp, err = req.Post("/api/user/orders")
	log.Println(resp.StatusCode(), err)

	log.Print(httpc.R().Header.Get("Content-Type"))
}

func generateOrderNumber() (string, error) {
	ds := DigitString(5, 15)
	cd, err := luhnCheckDigit(ds)
	if err != nil {
		return "", fmt.Errorf("cannot calculate check digit: %s", err)
	}
	return ds + strconv.Itoa(cd), nil
}

func luhnCheckDigit(s string) (int, error) {
	number, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}

	checkNumber := luhnChecksum(number)

	if checkNumber == 0 {
		return 0, nil
	}
	return 10 - checkNumber, nil
}

func luhnChecksum(number int) int {
	var luhn int

	for i := 0; number > 0; i++ {
		cur := number % 10

		if i%2 == 0 { // even
			cur = cur * 2
			if cur > 9 {
				cur = cur%10 + cur/10
			}
		}

		luhn += cur
		number = number / 10
	}
	return luhn % 10
}

func DigitString(minLen, maxLen int) string {
	var letters = "0123456789"

	slen := rnd.Intn(maxLen-minLen) + minLen

	s := make([]byte, 0, slen)
	i := 0
	for len(s) < slen {
		idx := rnd.Intn(len(letters) - 1)
		char := letters[idx]
		if i == 0 && '0' == char {
			continue
		}
		s = append(s, char)
		i++
	}

	return string(s)
}

func ASCIIString(minLen, maxLen int) string {
	var letters = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFJHIJKLMNOPQRSTUVWXYZ"

	slen := rnd.Intn(maxLen-minLen) + minLen

	s := make([]byte, 0, slen)
	i := 0
	for len(s) < slen {
		idx := rnd.Intn(len(letters) - 1)
		char := letters[idx]
		if i == 0 && '0' <= char && char <= '9' {
			continue
		}
		s = append(s, char)
		i++
	}

	return string(s)
}
