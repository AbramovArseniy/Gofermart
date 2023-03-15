package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"strings"

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
	registerOrderQuery     string = "INSERT INTO items (order_number, description, price) VALUES ($1, $2, $3)"
	registerGoodsQuery     string = "INSERT INTO goods (match, reward, reward_type) VALUES ($1, $2, $3)"
	registerOrderInfoQuery string = "INSERT INTO accrual (order_number, status) VALUES ($1, $2)"
	updateOrderInfoQuery   string = "UPDATE accrual SET status = $1, accrual = $2 WHERE order_number = $3"
	checkOrderStatusQuery  string = "SELECT EXISTS(SELECT status FROM accrual WHERE order_number = $1)"
	findOrderQuery         string = "SELECT EXISTS(SELECT order_number FROM items WHERE order_number = $1)"
	findGoodsQuery         string = "SELECT EXISTS(SELECT match FROM goods WHERE match = $1)"
	selectingGoodsQuery    string = "SELECT  match, reward, reward_type FROM goods"
	orderInfoQuery         string = "SELECT * FROM accrual WHERE order_number = $1"
)

func New(ctx context.Context, dba string) (*DataBase, error) {
	if dba == "" {
		err := fmt.Errorf("there is no DB address")
		return nil, err
	}
	db, err := sql.Open("pgx", dba)
	if err != nil {
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

	err := row.Scan(&order.Order, &order.Status, &order.Accrual)
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

	err := row.Scan(&exist)
	if err != nil {
		return false
	}

	return exist
}

func (d *DataBase) CheckOrderStatus(number string) bool {
	var status string

	if d.db == nil {
		return false
	}

	row := d.db.QueryRowContext(d.ctx, checkOrderStatusQuery, number)

	err := row.Scan(&status)
	if err != nil {
		return false
	}

	return status == string(types.StatusProcessing)
}

func (d *DataBase) RegisterOrder(order types.CompleteOrder) error {
	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return err
	}

	err := d.regOrderInfo(order.Order)
	if err != nil {
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
		_, err := stmt.ExecContext(d.ctx, order.Order, v.Description, v.Price)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (d *DataBase) UpdateOrderStatus(info types.OrdersInfo) error {

	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return err
	}

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(d.ctx, updateOrderInfoQuery)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.ExecContext(d.ctx, info.Status, info.Accrual, info.Order)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (d *DataBase) regOrderInfo(number string) error {
	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return err
	}

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(d.ctx, registerOrderInfoQuery)
	if err != nil {
		return err
	}

	defer stmt.Close()

	_, err = stmt.ExecContext(d.ctx, number, types.StatusRegistred)
	if err != nil {
		err = fmt.Errorf("exec: %s", err)
		return err
	}

	return tx.Commit()
}

func (d *DataBase) FindGoods(order types.CompleteOrder) (int, error) {
	var (
		accrual  int
		faccrual float32
	)

	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return accrual, err
	}

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return accrual, err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(d.ctx, selectingGoodsQuery)
	if err != nil {
		return accrual, err
	}

	defer stmt.Close()

	for _, v := range order.Goods {
		rows, err := stmt.QueryContext(d.ctx)
		if err != nil {
			return accrual, err
		}

		defer rows.Close()

		for rows.Next() {
			var item types.Goods

			err = rows.Scan(&item.Match, &item.Reward, &item.RewardType)
			if err != nil {
				return accrual, err
			}

			if strings.Contains(v.Description, item.Match) {
				switch item.RewardType {
				case "%":
					faccrual += v.Price / 100 * float32(item.Reward)
				case "pt":
					accrual += item.Reward
				}
			}
		}
		if rows.Err() != nil {
			return accrual, err
		}
	}

	accrual += int(faccrual)

	return accrual, tx.Commit()
}

func (d *DataBase) RegisterGoods(goods types.Goods) error {
	if d.db == nil {
		err := fmt.Errorf("you haven`t opened the database connection")
		return err
	}

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	stmt, err := tx.PrepareContext(d.ctx, registerGoodsQuery)
	if err != nil {
		return err
	}

	_, err = stmt.ExecContext(d.ctx, goods.Match, goods.Reward, goods.RewardType)
	if err != nil {
		err = fmt.Errorf("exec: %s", err)
		return err
	}
	defer stmt.Close()

	return tx.Commit()
}

func (d *DataBase) CheckGoods(match string) bool {
	var exist bool

	if d.db == nil {
		return false
	}

	row := d.db.QueryRowContext(d.ctx, findGoodsQuery, match)

	err := row.Scan(&exist)
	if err != nil {
		return false
	}

	return exist
}
