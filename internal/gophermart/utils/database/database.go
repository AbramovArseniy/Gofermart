package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"path"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrUserExists   = errors.New("such user already exist in DB")
	ErrScanData     = errors.New("error while scan user ID")
	ErrInvalidData  = errors.New("error user data is invalid")
	ErrHashGenerate = errors.New("error can't generate hash")
	ErrKeyNotFound  = errors.New("error user ID not found")
	ErrAlarm        = errors.New("error tx.BeginTx alarm")
	ErrAlarm2       = errors.New("error tx.PrepareContext alarm")

	selectOrdersByUserStmt            string        = `SELECT * FROM orders WHERE login=$1`
	updateOrderStatusToProcessingStmt string        = `UPDATE orders SET order_status='PROCESSING' WHERE order_num=$1`
	updateOrderStatusToProcessedStmt  string        = `UPDATE orders SET order_status='PROCESSED', accrual=$1 WHERE order_num=$2`
	updateOrderStatusToInvalidStmt    string        = `UPDATE orders SET order_status='INVALID' WHERE order_num=$1`
	updateOrderStatusToUnknownStmt    string        = `UPDATE orders SET order_status='UNKNOWN' WHERE order_num=$1`
	selectUserStmt                    string        = `SELECT id, login, password_hash FROM users WHERE login = $1`
	selectNotProcessedOrdersStmt      string        = `SELECT order_num FROM orders WHERE order_status='NEW' OR order_status='PROCESSING'`
	selectAccrualBalanceOrdersStmt    string        = `SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE order_status = 'PROCESSED' AND login = $1`
	selectAccrualWithdrawnStmt        string        = `SELECT COALESCE(SUM(accrual), 0) FROM withdrawals WHERE login = $1`
	insertWirdrawalStmt               string        = "INSERT INTO withdrawals (login, order_num, accrual, created_at) VALUES ($1, $2, $3, $4)"
	selectWithdrawalsByUserStmt       string        = `SELECT order_num, accrual, created_at FROM withdrawals WHERE login=$1`
	selectUserIDByOrderNumStmt        string        = `SELECT login FROM orders WHERE EXISTS(SELECT login FROM orders WHERE order_num = $1);`
	selectUserIDStmt                  string        = `SELECT login from orders WHERE order_num = $1;`
	checkUserDatastmt                 string        = `SELECT EXISTS(SELECT login, password_hash FROM users WHERE login = $1 AND password_hash = $2)`
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
	_, err := d.db.ExecContext(d.ctx, `CREATE TABLE IF NOT EXISTS users (
		id SERIAL UNIQUE,
		login VARCHAR UNIQUE NOT NULL,
		password_hash VARCHAR NOT NULL
	);`)
	if err != nil {
		log.Printf("error during create users %s", err)
	}

	_, err = d.db.ExecContext(d.ctx, `CREATE TABLE IF NOT EXISTS orders (
		order_num VARCHAR(255) PRIMARY KEY,
		login VARCHAR(16) NOT NULL,
		order_status VARCHAR(16) NOT NULL,
		accrual FLOAT,
		date_time TIMESTAMP NOT NULL
	);`)
	if err != nil {
		log.Printf("error during create orders %s", err)
	}

	_, err = d.db.ExecContext(d.ctx, `CREATE TABLE IF NOT EXISTS withdrawals (
		id serial primary key,
		login VARCHAR(16) NOT NULL,
		order_num VARCHAR(255) NOT NULL,
		accrual FLOAT NOT NULL,
		created_at TIMESTAMP NOT NULL
	);`)
	if err != nil {
		log.Printf("error during create withdrawals %s", err)
	}
}

func (d *DataBase) UpgradeOrderStatus(body []byte, orderNum string) error {
	var o types.Order

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
	switch o.Status {
	case "PROCESSING":
		_, err = updateOrderStatusToProcessingStmt.Exec(orderNum)
	case "REGISTERED":
		_, err = updateOrderStatusToProcessingStmt.Exec(orderNum)
	case "INVALID":
		_, err = updateOrderStatusToInvalidStmt.Exec(orderNum)
	case "PROCESSED":
		_, err = updateOrderStatusToProcessedStmt.Exec(o.Accrual, orderNum)
	default:
		_, err = updateOrderStatusToProcessedStmt.Exec(o.Accrual, orderNum)
	}
	if err != nil {
		log.Println("error updating orders status to db:", err)
		return fmt.Errorf("error inserting data to db: %w", err)
	}
	return tx.Commit()
}

func (d *DataBase) GetBalance(authUserLogin string) (float64, float64, error) {
	var order, withdrawn float64

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return order, withdrawn, err
	}

	defer tx.Rollback()

	selectAccrualBalanceOrdersStmt, err := tx.PrepareContext(d.ctx, selectAccrualBalanceOrdersStmt)
	if err != nil {
		return order, withdrawn, err
	}

	defer selectAccrualBalanceOrdersStmt.Close()

	err = selectAccrualBalanceOrdersStmt.QueryRow(authUserLogin).Scan(&order)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot select accrual sum from order database: %w", err)
	}
	order = Round(order, 0.01)
	selectAccrualWithdrawnStmt, err := tx.PrepareContext(d.ctx, selectAccrualWithdrawnStmt)
	if err != nil {
		return order, withdrawn, err
	}

	defer selectAccrualWithdrawnStmt.Close()

	err = selectAccrualWithdrawnStmt.QueryRow(authUserLogin).Scan(&withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("cannot select accrual sum from withdrawals database: %w", err)
	}
	balance := order - withdrawn

	return balance, withdrawn, nil
}

func (d *DataBase) SaveWithdrawal(w types.Withdrawal, authUserLogin string) error {
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

	_, err = insertWirdrawalStmt.Exec(authUserLogin, w.OrderNum, w.Accrual, time.Now().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("PostWithdrawalHandler: error while insert data into database: %w", err)
	}
	return tx.Commit()
}

func (d *DataBase) GetWithdrawalsByUser(authUserLogin string) ([]types.Withdrawal, bool, error) {
	var w []types.Withdrawal

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

	rows, err := selectWithdrawalsByUserStmt.QueryContext(d.ctx, authUserLogin)
	if err != nil {
		log.Println("error while selecting withdrawals from database:", err)
		return nil, false, fmt.Errorf("error while selecting withdrawals from database: %w", err)
	}
	for rows.Next() {
		var withdrawal types.Withdrawal
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

func (d *DataBase) CheckOrders(accrualSysClient types.Client) {
	ticker := time.NewTicker(checkOrderInterval)

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		log.Printf("error in checkorder func: %s", err)
	}

	defer tx.Rollback()

	selectNotProcessedOrdersStmt, err := tx.PrepareContext(d.ctx, selectNotProcessedOrdersStmt) // ОШИБКА ЗДЕСЬ
	if err != nil {
		log.Printf("error in checkorder func2: %s", err)
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
			url := accrualSysClient.URL
			url.Path = path.Join(accrualSysClient.URL.Path, orderNum)
			resp, err := accrualSysClient.Client.Get(url.String())
			if err != nil {
				log.Println("can't get response from accrual system:", err)
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("can't read body of reponsefrom accrual system:", err)
				return
			}
			d.UpgradeOrderStatus(body, orderNum)
		}
		if rows.Err() != nil {
			log.Println("CheckOrders: error while reading rows")
		}
	}
}

func Round(x, unit float64) float64 {
	return math.Round(x/unit) * unit
}

func (d *DataBase) SaveOrder(order *types.Order) error {
	_, err := d.db.ExecContext(d.ctx, `INSERT INTO orders (order_num, login, order_status, accrual, date_time) VALUES ($1, $2, $3, $4, $5)`,
		order.Number, order.User, order.Status, order.Accrual, time.Now())
	if err != nil {
		return err
	}

	return nil
}

func (d *DataBase) GetOrderUserByNum(orderNum string) (user string, exists bool, err error) {
	exists = false

	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return user, exists, err
	}

	defer tx.Rollback()

	selectUserIDByOrderNumStmt, err := tx.PrepareContext(d.ctx, selectUserIDByOrderNumStmt)
	if err != nil {
		return user, exists, err
	}

	defer selectUserIDByOrderNumStmt.Close()

	row := selectUserIDByOrderNumStmt.QueryRowContext(d.ctx, orderNum)

	err = row.Scan(&user)
	if !errors.Is(err, sql.ErrNoRows) {
		exists = true
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return user, exists, err
	}

	return user, exists, nil
}

func (d *DataBase) GetOrderUser(orderNum string) (userID string, err error) {
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return "", ErrInvalidData
	}
	defer tx.Rollback()

	selectUserIDStmt, err := tx.PrepareContext(d.ctx, selectUserIDStmt)
	if err != nil {
		return "", ErrInvalidData
	}
	defer selectUserIDStmt.Close()

	row := selectUserIDStmt.QueryRowContext(d.ctx, orderNum)
	err = row.Scan(&userID)
	if err != nil {
		return "", ErrInvalidData
	}
	return userID, nil
}

func (d *DataBase) GetOrdersByUser(authUserLogin string) ([]types.Order, bool, error) {
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		return nil, false, fmt.Errorf("GetOrdersByUser: error while BeginTx: %w", err)
	}

	defer tx.Rollback()

	selectOrdersByUserStmt, err := tx.PrepareContext(d.ctx, selectOrdersByUserStmt)
	if err != nil {
		return nil, false, fmt.Errorf("GetOrdersByUser: error while BeginTx: %w", err)
	}

	defer selectOrdersByUserStmt.Close()
	rows, err := selectOrdersByUserStmt.Query(authUserLogin)
	if err != nil {
		return nil, false, fmt.Errorf("GetOrdersByUser: error while selectOrdersByUserStmt.Query: %w", err)
	}
	var orders []types.Order
	for rows.Next() {
		var order types.Order
		var accrual float64
		err = rows.Scan(&order.Number, &order.User, &order.Status, &accrual, &order.UploadedAt)
		if err != nil {
			return nil, false, fmt.Errorf("GetOrdersByUser: error while scanning rows from database: %w", err)
		}
		order.Accrual = Round(accrual, 0.01)
		orders = append(orders, order)
	}
	log.Printf("GetOrdersByUser: ORDERS: %v", orders)
	if rows.Err() != nil {
		return nil, false, fmt.Errorf("GetOrdersByUser: rows.Err() error database: %w", err)
	}
	if len(orders) == 0 {
		return nil, false, nil
	}

	return orders, true, nil
}

func (d *DataBase) Close() {
	d.db.Close()
}

func (d *DataBase) CheckUserData(login, hash string) bool {
	var exist bool
	tx, err := d.db.BeginTx(d.ctx, nil)
	if err != nil {
		log.Printf("error while creating tx %s", err)
		return false
	}

	defer tx.Rollback()

	checkUserDatastmt, err := tx.PrepareContext(d.ctx, checkUserDatastmt)
	if err != nil {
		log.Printf("error while creating stmt %s", err)
		return false
	}

	defer checkUserDatastmt.Close()

	row := checkUserDatastmt.QueryRowContext(d.ctx, login, hash)
	err = row.Scan(&exist)
	if errors.Is(err, sql.ErrNoRows) {
		log.Println(err)
		return exist
	}
	if err != nil {
		log.Println(err)
		return exist
	}
	return exist
}

func (d *DataBase) RegisterNewUser(login string, password string) (types.User, error) {
	user := types.User{
		Login:        login,
		HashPassword: password,
	}
	query := `INSERT INTO users (login, password_hash) VALUES ($1, $2) returning id`
	row := d.db.QueryRowContext(context.Background(), query, login, password)
	if err := row.Scan(&user.ID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return types.User{}, ErrKeyNotFound
		}
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			if pgErr.Code == pgerrcode.UniqueViolation {
				return types.User{}, ErrUserExists
			}
		}
		return types.User{}, ErrScanData
	}

	return user, nil
}

func (d *DataBase) GetUserData(login string) (types.User, error) {
	var user types.User

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
	if errors.Is(err, pgx.ErrNoRows) {
		return types.User{}, nil
	}

	return user, err
}
