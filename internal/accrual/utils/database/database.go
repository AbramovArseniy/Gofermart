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

var orderInfoQuery string = "SELECT * FROM accrual WHERE order_number = $1"

type DataBase struct {
	db  *sql.DB
	ctx context.Context
	dba string
}

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
		//	cfg: cfg,
		dba: dba,
	}, nil
}

func (d *DataBase) Migrate() {

	driver, err := postgres.WithInstance(d.db, &postgres.Config{})
	if err != nil {
		log.Println(err)
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://./internal/utils/dbsaver/migrations",
		d.dba,
		driver)
	if err != nil {
		log.Println(err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Println(err)
	}

}

func (d *DataBase) RegisterNewOrder() error {
	return nil
}

func (d *DataBase) DBRegRequst() error {
	return nil
}

func (d *DataBase) DBGetOrderAccrual() error {
	return nil
}

func (d *DataBase) DBSetNewGoods() error {
	return nil
}

func (d *DataBase) GetOrderInfo(number string) (types.OrdersInfo, error) {
	var order types.OrdersInfo
	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return order, err
	}

	rows, err := d.db.QueryContext(d.ctx, orderInfoQuery, number)
	if err != nil {
		return order, err
	}

	err = rows.Scan(&order.Accrual, &order.Order, &order.Status)
	if err != nil {
		return order, err
	}

	err = rows.Err()
	if err != nil {
		return order, err
	}

	return order, nil
}
