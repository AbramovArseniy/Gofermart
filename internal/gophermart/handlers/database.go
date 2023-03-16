package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	selectOrdersByUserStmt string = `SELECT (order_num, order_status, accrual, date_time) FROM orders WHERE user_id=$1`
	// selectOrderByNumStmt              string        = `SELECT ( status, accrual, user_id) FROM orders WHERE order_num=$1`
	insertOrderStmt                   string        = `INSERT INTO orders (user_id, order_num, order_status, accrual, date_time) VALUES ($1, $2, $3, $4, $5)`
	updateOrderStatusToProcessingStmt string        = `UPDATE orders SET order_status='PROCESSING' WHERE order_num=$1`
	updateOrderStatusToProcessedStmt  string        = `UPDATE orders SET order_status='PROCESSED', accrual=$1 WHERE order_num=$2`
	updateOrderStatusToInvalidStmt    string        = `UPDATE orders SET order_status='INVALID' WHERE order_num=$1`
	updateOrderStatusToUnknownStmt    string        = `UPDATE orders SET order_status='UNKNOWN' WHERE order_num=$1`
	selectNotProcessedOrdersStmt      string        = `SELECT (order_num) FROM orders WHERE order_status='NEW' OR order_status='PROCESSING'`
	selectBalacneAndWithdrawnStmt     string        = `SELECT SUM(accrual) AS accrual_sum from orders where order_status = 'PROCESSED' and user_id = $1 UNION SELECT SUM(accrual) FROM withdrawals WHERE user_id = $1;`
	insertWirdrawalStmt               string        = "INSERT INTO withdrawals (user_id, order_num, accrual, created_at) VALUES ($1, $2, $3, $4)"
	selectWithdrawalsByUserStmt       string        = `SELECT (order_num, accrual, created_at) FROM withdrawals WHERE user_id=$1`
	insertUserStmt                    string        = `INSERT INTO users (login, password_hash) VALUES ($1, $2) returning id`
	selectUserStmt                    string        = `SELECT id, login, password_hash FROM users WHERE login = $1`
	selectUserIdByOrderNumStmt        string        = `SELECT user_id FROM orders WHERE EXISTS(SELECT user_id FROM orders WHERE order_num = $1);`
	checkOrderInterval                time.Duration = 5 * time.Second
)

type DataBase struct {
	db  *sql.DB
	ctx context.Context
	dba string
}

func NewDataBase(ctx context.Context, dba string) (*DataBase, error) {
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
		"file://internal/gophermart/migrations",
		d.dba,
		driver)
	if err != nil {
		log.Println(err)
	}

	if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Println(err)
	}

}

func (d *DataBase) UpgradeOrderStatus(accrualSysClient Client, orderNum string) error {
	body, err := accrualSysClient.DoRequest(orderNum)
	if err != nil {
		return fmt.Errorf("error while getting response body from accrual system: %w", err)
	}

	var o Order

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	updateOrderStatusToProcessingStmt, err := tx.PrepareContext(d.ctx, updateOrderStatusToProcessingStmt)
	if err != nil {
		return err
	}

	updateOrderStatusToInvalidStmt, err := tx.PrepareContext(d.ctx, updateOrderStatusToInvalidStmt)
	if err != nil {
		return err
	}

	updateOrderStatusToProcessedStmt, err := tx.PrepareContext(d.ctx, updateOrderStatusToProcessedStmt)
	if err != nil {
		return err
	}

	updateOrderStatusToUnknownStmt, err := tx.PrepareContext(d.ctx, updateOrderStatusToUnknownStmt)
	if err != nil {
		return err
	}
	defer updateOrderStatusToInvalidStmt.Close()
	defer updateOrderStatusToProcessingStmt.Close()
	defer updateOrderStatusToProcessedStmt.Close()
	defer updateOrderStatusToUnknownStmt.Close()

	err = json.Unmarshal(body, &o)
	if err != nil {
		log.Println("failed to unmarshal json from response body from accrual system:", err)
		return fmt.Errorf("failed to unmarshal json from response body from accrual system: %w", err)
	}
	if o.Status == "PROCESSING" || o.Status == "REGISTERED" {
		_, err = updateOrderStatusToProcessingStmt.Exec(orderNum)
	} else if o.Status == "INVALID" {
		_, err = updateOrderStatusToInvalidStmt.Exec(orderNum)
	} else if o.Status == "PROCESSED" {
		_, err = updateOrderStatusToProcessedStmt.Exec(orderNum)
	} else {
		_, err = updateOrderStatusToUnknownStmt.Exec(orderNum)
	}
	if err != nil {
		log.Println("error inserting data to db:", err)
		return fmt.Errorf("error inserting data to db: %w", err)
	}
	return nil
}

func (d *DataBase) GetBalance(authUserID int) (float64, float64, error) {
	var balance, withdrawn float64

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return balance, withdrawn, err
	}

	defer tx.Rollback()

	selectBalacneAndWithdrawnStmt, err := tx.PrepareContext(d.ctx, selectBalacneAndWithdrawnStmt)
	if err != nil {
		return balance, withdrawn, err
	}

	defer selectBalacneAndWithdrawnStmt.Close()

	err = selectBalacneAndWithdrawnStmt.QueryRow(authUserID).Scan(&balance, &withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot select data from database: %w", err)
	}

	balance = balance - withdrawn

	return balance, withdrawn, nil
}

func (d *DataBase) SaveWithdrawal(w Withdrawal, authUserID int) error {
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	insertWirdrawalStmt, err := tx.PrepareContext(d.ctx, insertWirdrawalStmt)
	if err != nil {
		return err
	}

	defer insertWirdrawalStmt.Close()

	_, err = insertWirdrawalStmt.Exec(authUserID, w.OrderNum, w.Accrual, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("PostWithdrawalHandler: error while insert data into database: %w", err)
	}
	return nil
}

func (d *DataBase) GetWithdrawalsByUser(authUserID int) ([]Withdrawal, bool, error) {
	var w []Withdrawal

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return w, false, err
	}

	defer tx.Rollback()

	selectWithdrawalsByUserStmt, err := tx.PrepareContext(d.ctx, selectWithdrawalsByUserStmt)
	if err != nil {
		return w, false, err
	}

	defer selectWithdrawalsByUserStmt.Close()

	rows, err := selectWithdrawalsByUserStmt.QueryContext(d.ctx, authUserID)
	if err != nil {
		log.Println("error while selecting withdrawals from database:", err)
		return nil, false, fmt.Errorf("error while selecting withdrawals from database: %w", err)
	}
	for rows.Next() {
		var withdrawal Withdrawal
		err = rows.Scan(&withdrawal.OrderNum, &withdrawal.Accrual, &withdrawal.ProcessedAt)
		if err != nil {
			log.Println("error while scanning data:", err)
			return nil, false, fmt.Errorf("error while scanning data: %w", err)
		}
		w = append(w, withdrawal)
	}
	if rows.Err() != nil {
		log.Println("rows.Err:", err)
		return nil, false, fmt.Errorf("rows.Err: %w", err)
	}
	if len(w) == 0 {
		return nil, false, nil
	}
	return w, true, nil
}

func (d *DataBase) CheckOrders(accrualSysClient Client) {
	ticker := time.NewTicker(checkOrderInterval)

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		log.Printf("error in checkorder func: %s", err)
	}

	defer tx.Rollback()

	selectNotProcessedOrdersStmt, err := tx.PrepareContext(d.ctx, selectNotProcessedOrdersStmt) // ОШИБКА ЗДЕСЬ
	if err != nil {
		log.Printf("error in chechorder func2: %s", err)
	}

	defer selectNotProcessedOrdersStmt.Close()

	for {
		<-ticker.C
		rows, err := selectNotProcessedOrdersStmt.Query()
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		if err != nil {
			log.Println("CheckOrders: error while selecting data from Database")
			return
		}
		for rows.Next() {
			var orderNum string
			rows.Scan(&orderNum)
			d.UpgradeOrderStatus(accrualSysClient, orderNum)
		}
		if rows.Err() != nil {
			log.Println("CheckOrders: error while reading rows")
		}
	}

}

func (d *DataBase) RegisterNewUser(login string, password string) (User, error) {
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

	insertUserStmt, err := tx.PrepareContext(d.ctx, insertUserStmt)
	if err != nil {
		return User{}, ErrAlarm2
	}
	defer insertUserStmt.Close()
	log.Println("everything still is ok")
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

func (d *DataBase) GetUserData(login string) (User, error) {
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

func (d *DataBase) SaveOrder(authUserID int, order *Order) error {

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return err
	}

	defer tx.Rollback()

	insertOrderStmt, err := tx.PrepareContext(d.ctx, insertOrderStmt)
	if err != nil {
		return err
	}

	defer insertOrderStmt.Close()

	_, err = insertOrderStmt.ExecContext(d.ctx, authUserID, order.Number, order.Status, order.Accrual, order.UploadedAt)
	if err != nil {
		return err
	}

	return nil
}

func (d *DataBase) GetOrderUserByNum(orderNum string) (userID int, exists bool, err error) {
	exists = false

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return userID, exists, err
	}

	defer tx.Rollback()

	selectUserIdByOrderNumStmt, err := tx.PrepareContext(d.ctx, selectUserIdByOrderNumStmt)
	if err != nil {
		return userID, exists, err
	}

	defer selectUserIdByOrderNumStmt.Close()

	row := selectUserIdByOrderNumStmt.QueryRowContext(d.ctx, orderNum)

	err = row.Scan(&userID)
	if err != nil {
		return userID, exists, err
	}

	if userID != 0 {
		exists = true
	}

	return userID, exists, nil
}

func (d *DataBase) GetOrdersByUser(authUserID int) (orders []Order, exist bool, err error) {
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return
	}

	defer tx.Rollback()

	selectOrdersByUserStmt, err := tx.PrepareContext(d.ctx, selectOrdersByUserStmt)
	if err != nil {
		return
	}

	defer selectOrdersByUserStmt.Close()

	rows, err := selectOrdersByUserStmt.Query(authUserID)
	if err != nil {
		return nil, false, fmt.Errorf("error while getting orders by user from database: %w", err)
	}
	for rows.Next() {
		var order Order
		err = rows.Scan(&order.Number, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, false, fmt.Errorf("error while scanning rows from database: %w", err)
		}
		orders = append(orders, order)
	}
	if rows.Err() != nil {
		return nil, false, fmt.Errorf("rows.Err() error database: %w", err)
	}
	if len(orders) == 0 {
		return nil, false, nil
	}

	return orders, true, nil
}

func (d *DataBase) Close() {
	d.db.Close()
}
