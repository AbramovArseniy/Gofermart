package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/handlers"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func SetGophermartParams() (address, accrualSysAddress string, db *sql.DB) {
	var databaseURI string
	var flagAddress, flagDatabaseURI, flagAccrualSysAddress string
	flag.StringVar(&flagAddress, "-a", "localhost:8080", "server_address")
	flag.StringVar(&flagDatabaseURI, "-d", "", "database_uri")
	flag.StringVar(&flagDatabaseURI, "-r", "localhost:8000", "database_uri")
	address, set := os.LookupEnv("RUN_ADDRESS")
	if !set {
		address = flagAddress
	}
	databaseURI, set = os.LookupEnv("DATABASE_URI")
	if !set {
		databaseURI = flagDatabaseURI
	}
	db, err := sql.Open("pgx", databaseURI)
	if err != nil {
		log.Println("error while opening database:", err)
		db = nil
	}
	accrualSysAddress, set = os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS")
	if !set {
		accrualSysAddress = flagAccrualSysAddress
	}
	return
}

func setDatabase(db *sql.DB) error {
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("could not create driver: %w", err)
	}
	m, err := migrate.NewWithDatabaseInstance(
		"file://./migrations",
		"postgres", driver)
	if err != nil {
		return fmt.Errorf("could not create migration: %w", err)
	}

	if err != nil {
		log.Println("error while creating table:", err)
		return err
	}
	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return err
	}
	return nil
}

func main() {
	g := handlers.NewGophermart(SetGophermartParams())
	defer g.Database.Close()
	if g.Database == nil {
		panic("no db opened")
	}
	err := setDatabase(g.Database)
	if err != nil {
		log.Println("error seting database:", err)
	}
	go g.CheckOrders()
	r := g.Router()
	log.Println("Server started at", g.Address)
	err = http.ListenAndServe(g.Address, r)
	if err != nil {
		log.Fatal("error while starting server: ", err)
	}
}
