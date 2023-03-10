package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/handlers"
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

func main() {
	gophermartAddr, accrualSysAddr, db := SetGophermartParams()
	g := handlers.NewGophermart(accrualSysAddr, db)
	defer g.Storage.Finish()
	if g.Storage == nil {
		log.Fatal("no db opened")
	}
	err := g.Storage.SetStorage()
	if err != nil {
		log.Println("error seting database:", err)
	}
	go g.Storage.CheckOrders(g.AccrualSysClient)
	r := g.Router()
	log.Println("Server started at", gophermartAddr)
	err = http.ListenAndServe(gophermartAddr, r)
	if err != nil {
		log.Fatal("error while starting server: ", err)
	}
}
