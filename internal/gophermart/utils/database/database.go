package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/services"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type DBStorage struct {
	db *sql.DB
}

func NewUserDataBase(databasePath string) (services.UserDB, error) {
	db, err := sql.Open("pgx", databasePath)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
		CREATE TABLE  users (
			id SERIAL PRIMARY KEY,
			login VARCHAR UNIQUE NOT NULL,
			password_hash VARCHAR NOT NULL,
		)
		`)
	if err != nil {
		return nil, fmt.Errorf("unable to CREATE TABLE in DB: %w", err)
	}
	return &DBStorage{db: db}, nil
}

func (d *DBStorage) RegisterNewUser(login string, password string) (services.User, error) {
	query := `INSERT INTO users (login, password_hash) VALUES ($1, $2) returning id`
	row := d.db.QueryRowContext(context.Background(), query, login, password)
	var user services.User
	err := row.Scan(&user.ID)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation {
			return services.User{}, services.ErrUserExists
		}
	}
	return user, nil
}

func (d *DBStorage) GetUserData(login string) (services.User, error) {
	var user services.User
	query := `SELECT id, login, password_hash FROM users WHERE login = $1`
	row := d.db.QueryRow(query, login)
	err := row.Scan(&user.ID, &user.Login, &user.HashPassword)
	if err == pgx.ErrNoRows {
		return services.User{}, nil
	}
	return user, err
}

// func (d *DBStorage) Registration(userdata services.UserData) (services.User, error) {
// 	var user services.User
// 	query := `INSERT INTO userdata (login, password) VALUES ($1, $2) returning id`
// 	_, err := d.db.Exec(query, userdata.Login, userdata.Password)
// 	if err != nil && strings.Contains(err.Error(), pgerrcode.UniqueViolation) {
// 		return user, ErrDataExists
// 	}
// 	return user, nil
// }

// func (d *DBStorage) CheckLoginUnicality(ctx context.Context, login string) (bool, error) {
// 	query := `SELECT EXISTS (SELECT 1 FROM userdata WHERE login = $1)`
// 	row := d.db.QueryRowContext(ctx, query, login)
// 	var check bool
// 	if err := row.Scan(&check); err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			return false, ErrKeyNotFound
// 		}
// 		return true, fmt.Errorf("unable to Scan login from DB (CheckLoginUnicality): %w", err)
// 	}
// 	return check, nil
// }

// func (d DataBaseStorage) Close() {
// 	d.DataBase.Close()
// }
