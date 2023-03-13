package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/AbramovArseniy/Gofermart/internal/accrual/utils/types"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	_ "github.com/jackc/pgx/v5/stdlib"
)

type DataBase struct {
	db  *sql.DB
	ctx context.Context
	dba string
}

var (
	orderInfoQuery     string = "SELECT * FROM accrual WHERE order_number = $1"
	findOrderQuery     string = "SELECT EXIST(SELECT order_number FROM accrual WHERE order_number = $1)"
	registerOrderQuery string = "INSERT INTO items (order_number, description, price) VALUES ($1, $2, $3)"
)

func New(ctx context.Context, dba string) (*DataBase, error) {
	if dba == "" {
		err := fmt.Errorf("there is no DB address")
		return nil, err
	}
	db, err := sql.Open("pgx", dba)
	if err != nil {
		log.Printf("Unable to connect to database: %v\n", err)
		return nil, err
	}
	return &DataBase{
		db:  db,
		ctx: ctx,
		dba: dba,
	}, nil
}

func (d *DataBase) Migrate() {

	driver, err := postgres.WithInstance(d.db, &postgres.Config{})
	if err != nil {
		log.Println(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://internal/accrual/utils/database/migrations",
		d.dba,
		driver)
	if err != nil {
		log.Println(err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Println(err)
	}

}

func (d *DataBase) GetOrderInfo(number string) (types.OrdersInfo, error) {
	var order types.OrdersInfo
	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return order, err
	}

	row := d.db.QueryRowContext(d.ctx, orderInfoQuery, number)

	err := row.Scan(&order.Accrual, &order.Order, &order.Status)
	if err != nil {
		return order, err
	}

	err = row.Err()
	if err != nil {
		return order, err
	}

	return order, nil
}

func (d *DataBase) FindOrder(number string) bool {
	var exist bool

	if d.db == nil {
		return false
	}

	row := d.db.QueryRowContext(d.ctx, findOrderQuery, number)

	err := row.Scan(exist)
	if err != nil {
		return false
	}

	return exist
}

func (d *DataBase) RegisterOrder(order types.CompleteOrder) error {

	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return err
	}

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(d.ctx, registerOrderQuery)
	if err != nil {
		return err
	}

	defer stmt.Close()

	for _, v := range order.Goods {
		log.Printf("order desc %s", v.Description)
		_, err := stmt.ExecContext(d.ctx, order.Order, v.Description, v.Price)
		if err != nil {
			log.Printf("during insert was a %s", err)
			return err
		}
	}

	return tx.Commit()
}
