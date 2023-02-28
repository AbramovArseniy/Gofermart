package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/handlers"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/config"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/storage"
)

func main() {
	var storageMock *storage.Storage
	config := config.New()
	handler := handlers.New(storageMock)

	router := chi.NewRouter()
	router.Mount("/", handler.Route())

	log.Fatal(http.ListenAndServe(config.Address, router))
}
