package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"path"
	"time"

	"github.com/AbramovArseniy/Gofermart/internal/gophermart/utils/types"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	// "github.com/jackc/pgconn"
	// "github.com/jackc/pgerrcode"
	// "github.com/jackc/pgx"
	"github.com/jackc/pgconn"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var (
	ErrUserExists = errors.New("such user already exist in DB")
	// ErrNewRegistration = errors.New("error while register user - main problem")
	ErrScanData     = errors.New("error while scan user ID")
	ErrInvalidData  = errors.New("error user data is invalid")
	ErrHashGenerate = errors.New("error can't generate hash")
	ErrKeyNotFound  = errors.New("error user ID not found")
	ErrAlarm        = errors.New("error tx.BeginTx alarm")
	ErrAlarm2       = errors.New("error tx.PrepareContext alarm")

	selectOrdersByUserStmt string = `SELECT * FROM orders WHERE login=$1`
	// selectOrderByNumStmt              string        = `SELECT ( status, accrual, user_id) FROM orders WHERE order_num=$1`
	//	insertOrderStmt                   string = `INSERT INTO orders (order_num, user_id, order_status, accrual, date_time) VALUES ($1, $2, $3, $4, $5)`
	updateOrderStatusToProcessingStmt string = `UPDATE orders SET order_status='PROCESSING' WHERE order_num=$1`
	updateOrderStatusToProcessedStmt  string = `UPDATE orders SET order_status='PROCESSED', accrual=$1 WHERE order_num=$2`
	updateOrderStatusToInvalidStmt    string = `UPDATE orders SET order_status='INVALID' WHERE order_num=$1`
	updateOrderStatusToUnknownStmt    string = `UPDATE orders SET order_status='UNKNOWN' WHERE order_num=$1`
	selectUserStmt                    string = `SELECT id, login, password_hash FROM users WHERE login = $1`
	selectNotProcessedOrdersStmt      string = `SELECT order_num FROM orders WHERE order_status='NEW' OR order_status='PROCESSING'`
	selectAccrualBalanceOrdersStmt    string = `SELECT COALESCE(SUM(accrual), 0) FROM orders where order_status = 'PROCESSED' and login = $1`
	selectAccrualWithdrawnStmt        string = `SELECT COALESCE(SUM(accrual), 0) FROM withdrawals WHERE login = $1`
	insertWirdrawalStmt               string = "INSERT INTO withdrawals (login, order_num, accrual, created_at) VALUES ($1, $2, $3, $4)"
	selectWithdrawalsByUserStmt       string = `SELECT (order_num, accrual, created_at) FROM withdrawals WHERE login=$1`
	// insertUserStmt                    string        = `INSERT INTO users (login, password_hash) VALUES ($1, $2) returning id`
	// selectUserStmt                    string        = `SELECT id, login, password_hash FROM users WHERE login = $1`
	selectUserIDByOrderNumStmt string        = `SELECT login FROM orders WHERE EXISTS(SELECT login FROM orders WHERE order_num = $1);`
	checkUserDatastmt          string        = `SELECT EXISTS(SELECT login, password_hash FROM users WHERE login = $1 AND password_hash = $2)`
	checkOrderInterval         time.Duration = 5 * time.Second
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

	// driver, err := postgres.WithInstance(d.db, &postgres.Config{})
	// if err != nil {
	// 	log.Println(err)
	// }

	// m, err := migrate.NewWithDatabaseInstance(
	// 	"file://migrations",
	// 	d.dba,
	// 	driver)
	// if err != nil {
	// 	log.Println(err)
	// }

	// log.Println(m.Version())

	// if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
	// 	log.Println(err)
	// }

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
	log.Printf(`got order from accrual:
	status: %s
	accrual: %d`, o.Status, o.Accrual)
	if o.Status == "PROCESSING" || o.Status == "REGISTERED" {
		_, err = updateOrderStatusToProcessingStmt.Exec(orderNum)
	} else if o.Status == "INVALID" {
		_, err = updateOrderStatusToInvalidStmt.Exec(orderNum)
	} else if o.Status == "PROCESSED" {
		_, err = updateOrderStatusToProcessedStmt.Exec(o.Accrual, orderNum)
	} else {
		_, err = updateOrderStatusToUnknownStmt.Exec(orderNum)
	}
	if err != nil {
		log.Println("error updating orders status to db:", err)
		return fmt.Errorf("error inserting data to db: %w", err)
	}

	return nil
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
	return nil
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
			url := accrualSysClient.URL
			url.Path = path.Join(accrualSysClient.URL.Path, orderNum)
			resp, err := accrualSysClient.Client.Get(url.String())
			if err != nil {
				log.Println("can't get response from accrual sytem:", err)
				return
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				log.Println("can't read body of reponsefrom accrual sytem:", err)
				return
			}
			d.UpgradeOrderStatus(body, orderNum)
		}
		if rows.Err() != nil {
			log.Println("CheckOrders: error while reading rows")
		}
	}

}

// func (d *DataBase) RegisterNewUser(login string, password string) (User, error) {
// 	user := User{
// 		Login:        login,
// 		HashPassword: password,
// 	}
// 	log.Printf("user: %+v", user)
// 	tx, err := d.db.BeginTx(d.ctx, nil)
// 	if err != nil {
// 		return User{}, ErrAlarm
// 	}
// 	defer tx.Rollback()

// 	insertUserStmt, err := tx.PrepareContext(d.ctx, insertUserStmt)
// 	if err != nil {
// 		return User{}, ErrAlarm2
// 	}
// 	defer insertUserStmt.Close()
// 	log.Println("everything still is ok")
// 	row := insertUserStmt.QueryRowContext(d.ctx, user.Login, user.HashPassword)
// 	log.Printf("row: %+v", row)
// 	if err := row.Scan(&user.ID); err != nil {
// 		if errors.Is(err, sql.ErrNoRows) {
// 			return User{}, ErrKeyNotFound
// 		}
// 		return User{}, ErrScanData
// 	}
// 	log.Printf("user: %+v", user)
// 	var pgErr *pgconn.PgError
// 	if errors.As(err, &pgErr) {
// 		if pgErr.Code == pgerrcode.UniqueViolation {
// 			return User{}, ErrUserExists
// 		}
// 	}
// 	return user, nil
// }

// func (d *DataBase) GetUserData(login string) (User, error) {
// 	var user User

// 	tx, err := d.db.BeginTx(d.ctx, nil)
// 	if err != nil {
// 		return user, err
// 	}
// 	defer tx.Rollback()

// 	selectUserStmt, err := tx.PrepareContext(d.ctx, selectUserStmt)
// 	if err != nil {
// 		return user, err
// 	}
// 	defer selectUserStmt.Close()

// 	row := selectUserStmt.QueryRow(login)
// 	err = row.Scan(&user.ID, &user.Login, &user.HashPassword)
// 	if err == pgx.ErrNoRows {
// 		return User{}, nil
// 	}
// 	return user, err
// }

func (d *DataBase) SaveOrder(order *types.Order) error {
	_, err := d.db.ExecContext(d.ctx, `INSERT INTO orders (order_num, login, order_status, accrual, date_time) VALUES ($1, $2, $3, $4, $5)`,
		order.Number, order.User, order.Status, order.Accrual, time.Now())
	if err != nil {
		return err
	}
	// tx, err := d.db.BeginTx(d.ctx, nil)
	// if err != nil {
	// 	return err
	// }

	// defer tx.Rollback()

	// insertOrderStmt, err := tx.PrepareContext(d.ctx, insertOrderStmt)
	// if err != nil {
	// 	return err
	// }

	// defer insertOrderStmt.Close()

	// _, err = insertOrderStmt.ExecContext(d.ctx, order.Number, order.UserID, order.Status, order.Accrual, order.UploadedAt)
	// if err != nil {
	// 	return err
	// }

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
	log.Println("GetOrdersByUser: EVERYTHING still is OK")
	rows, err := selectOrdersByUserStmt.Query(authUserLogin)
	if err != nil {
		return nil, false, fmt.Errorf("GetOrdersByUser: error while selectOrdersByUserStmt.Query: %w", err)
	}
	log.Println("GetOrdersByUser: EVERYTHING still is OK #2")
	var orders []types.Order
	for rows.Next() {
		var order types.Order
		err = rows.Scan(&order.Number, &order.User, &order.Status, &order.Accrual, &order.UploadedAt)
		if err != nil {
			return nil, false, fmt.Errorf("GetOrdersByUser: error while scanning rows from database: %w", err)
		}

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

	// // NEW VERSION
	// tx, err := d.db.Begin()
	// if err != nil {
	// 	return User{}, ErrAlarm
	// }
	// defer tx.Rollback()
	// log.Println("1: everything still is ok")
	// insertUserStmt, err := d.db.Prepare("INSERT INTO users (login, password_hash) VALUES ($1, $2) ON CONFLICT DO NOTHING RETURNING id")
	// txInsertUserStmt := tx.StmtContext(d.ctx, insertUserStmt)
	// log.Println("2: everything still ok")
	// row := txInsertUserStmt.QueryRowContext(d.ctx, user.Login, user.HashPassword)
	// if err := row.Scan(&user.ID); err != nil {
	// 	if errors.Is(err, sql.ErrNoRows) {
	// 		return User{}, ErrKeyNotFound
	// 	}
	// 	return User{}, ErrScanData
	// }
	// log.Printf("user after Insert: %+v", user)

	// OLD VERSION
	// tx, err := d.db.BeginTx(d.ctx, nil)
	// if err != nil {
	// 	return User{}, ErrAlarm
	// }
	// defer tx.Rollback()
	// insertUserStmt, err := tx.PrepareContext(d.ctx, insertUserStmt) // ОШИБКА ТУТ
	// if err != nil {
	// 	return User{}, ErrAlarm2
	// }
	// defer insertUserStmt.Close()
	// log.Println("2: everything still ok")
	// row := insertUserStmt.QueryRowContext(d.ctx, user.Login, user.HashPassword)
	// log.Printf("row: %+v", row)
	// if err := row.Scan(&user.ID); err != nil {
	// 	if errors.Is(err, sql.ErrNoRows) {
	// 		return User{}, ErrKeyNotFound
	// 	}
	// 	return User{}, ErrScanData
	// }
	// log.Printf("user: %+v", user)
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
	if err == pgx.ErrNoRows {
		return types.User{}, nil
	}
	return user, err
}
