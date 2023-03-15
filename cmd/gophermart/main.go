package main

import (
	"database/sql"
	"log"
	"net/http"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/handlers"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/config"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {

	cfg := config.New()
	db, err := sql.Open("pgx", cfg.DBAddress)
	if err != nil {
		log.Println("error while opening database:", err)
		db = nil
	}
	if db == nil {
		log.Fatal("no db opened")
	}

	database := handlers.NewDatabase(db)

	err = database.SetStorage(cfg.DBAddress)
	if db == nil {
		log.Fatal("can't set database:", err)
	}
	g := handlers.NewGophermart(cfg.Accrual, database, cfg.JWTSecret)
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
