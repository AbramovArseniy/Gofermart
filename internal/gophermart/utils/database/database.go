package database

import (
	// "context"
	"database/sql"
	// "errors"
	"fmt"
	"strings"

	"github.com/AbramovArseniy/Gofermart.git/internal/storage"
	"github.com/jackc/pgerrcode"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DataBaseStorage struct {
	DataBase *sql.DB
}

func NewDataBaseStorage(databasePath string) (storage.Maininterface, error) {
	db, err := sql.Open("pgx", databasePath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS userdata (
			id SERIAL PRIMARY KEY,
			login VARCHAR UNIQUE, 
			password VARCHAR,
			e_bally VARCHAR)
		`)
	if err != nil {
		return nil, fmt.Errorf("unable to CREATE TABLE in DB: %w", err)
	}
	return &DataBaseStorage{DataBase: db}, nil
}

func (d *DataBaseStorage) AddData(login, password string) error {
	query := `INSERT INTO userdata (login, password) VALUES ($1, $2)`
	_, err := d.DataBase.Exec(query, login, password)
	if err != nil && strings.Contains(err.Error(), pgerrcode.UniqueViolation) {
		return storage.ErrDataExists
	}
	return nil
}

func (d *DataBaseStorage) CheckLoginUnicality(login string) (error, bool) {
	query := `INSERT INTO urls (original_url, short_id, user_id, deleted) VALUES ($1, $2, $3, $4)`
	_, err := d.DataBase.Exec(query, u.OriginalURL, u.ShortURL, u.UserID, false)

	return nil, true
}
