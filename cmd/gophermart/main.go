package main

import (
	"context"
	"log"
	"net/http"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/handlers"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/config"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/database"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	context := context.Background()
	cfg := config.New()
	database, err := database.NewDataBase(context, cfg.DBAddress)
	if err != nil {
		log.Fatalf("Error during open db %s", err)
	}

	database.Migrate()

	auth := handlers.NewAuth(database, cfg.JWTSecret, context)
	g := handlers.NewGophermart(cfg.Accrual, cfg.JWTSecret, database, auth)
	defer g.Storage.Close()
	r := g.Router()
	s := http.Server{
		Addr:    cfg.Address,
		Handler: r,
	}
	log.Println("Server started at", cfg.Address)
	go g.Storage.CheckOrders(g.AccrualSysClient)
	err = s.ListenAndServe()
	if err != nil {
		log.Fatal("error while starting server: ", err)
	}
}
