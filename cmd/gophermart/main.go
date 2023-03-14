package main

import (
	"database/sql"
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/handlers"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/database"
	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/services"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func SetGophermartParams() (address, accrualSysAddress string, db *sql.DB, auth *services.AuthJWT) {
	var databaseURI, JWTSecret string
	var flagAddress, flagDatabaseURI, flagAccrualSysAddress, flagJWTSecret string
	flag.StringVar(&flagAddress, "a", "localhost:8080", "server_address")
	flag.StringVar(&flagDatabaseURI, "d", "", "database_uri")
	flag.StringVar(&flagDatabaseURI, "r", "localhost:8000", "database_uri")
	flag.StringVar(&flagJWTSecret, "js", "jwt secret token", "secret token for jwt") // added Albert
	address, set := os.LookupEnv("RUN_ADDRESS")
	if !set {
		address = flagAddress
	}
	databaseURI, set = os.LookupEnv("DATABASE_URI")
	if !set {
		databaseURI = flagDatabaseURI
	}
	JWTSecret, set = os.LookupEnv("JWT_SECRET") // added A
	if !set {
		JWTSecret = flagJWTSecret
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
	// added A
	userRepo, err := database.NewUserDataBase(databaseURI)
	if err != nil {
		log.Println("main: couldn't initialize user storage:", err)
	}
	auth = services.NewAuth(userRepo, JWTSecret)
	// end
	return
}

func main() {
	gophermartAddr, accrualSysAddr, db, auth := SetGophermartParams()
	g := handlers.NewGophermart(accrualSysAddr, db, auth)
	defer g.Storage.Close()
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
