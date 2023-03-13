package main

import (
	"context"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/handlers"
	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/config"
	db "github.com/AbramovArseniy/Gofermart/internal/accrual/utils/database"
)

func main() {
	context := context.Background()
	config := config.New()
	database, err := db.New(context, config.DBAddress)
	if err != nil {
		log.Fatal(err)
	}
	database.Migrate()

	handler := handlers.New(database)

	router := chi.NewRouter()
	router.Mount("/", handler.Route())

	log.Fatal(http.ListenAndServe(config.Address, router))
}
