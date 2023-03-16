package handlers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	insertUserStmt string = `INSERT INTO users (login, password_hash) VALUES ($1, $2) returning id`
	selectUserStmt string = `SELECT id, login, password_hash FROM users WHERE login = $1`
)

type DBStorage struct {
	db  *sql.DB
	ctx context.Context
	dba string
}

func NewUserDataBase(ctx context.Context, dba string) (UserDB, error) {
	if dba == "" {
		err := fmt.Errorf("there is no DB address")
		return nil, err
	}
	db, err := sql.Open("pgx", dba)
	if err != nil {
		return nil, err
	}
	return &DBStorage{
		db:  db,
		ctx: ctx,
		dba: dba,
	}, nil
}

func (d *DBStorage) RegisterNewUser(login string, password string) (User, error) {
	user := User{
		Login:        login,
		HashPassword: password,
	}
	log.Printf("user: %+v", user)
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return User{}, ErrAlarm
	}
	defer tx.Rollback()
	log.Println("1: everything still is ok")
	insertUserStmt, err := tx.PrepareContext(d.ctx, insertUserStmt)
	if err != nil {
		return User{}, ErrAlarm2
	}
	defer insertUserStmt.Close()
	log.Println("2: everything still ok")
	row := insertUserStmt.QueryRowContext(d.ctx, user.Login, user.HashPassword)
	log.Printf("row: %+v", row)
	if err := row.Scan(&user.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrKeyNotFound
		}
		return User{}, ErrScanData
	}
	log.Printf("user: %+v", user)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		if pgErr.Code == pgerrcode.UniqueViolation {
			return User{}, ErrUserExists
		}
	}
	return user, nil
}

func (d *DBStorage) GetUserData(login string) (User, error) {
	var user User

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return user, err
	}
	defer tx.Rollback()

	selectUserStmt, err := tx.PrepareContext(d.ctx, selectUserStmt)
	if err != nil {
		return user, err
	}
	defer selectUserStmt.Close()

	row := selectUserStmt.QueryRow(login)
	err = row.Scan(&user.ID, &user.Login, &user.HashPassword)
	if err == pgx.ErrNoRows {
		return User{}, nil
	}
	return user, err
}
