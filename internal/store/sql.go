package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"go.uber.org/zap"
)

// SQLStorage реализует интерфейс для работы с SQL-базой
// и реализует как UserStorage, так и OrderStorage

type SQLPinger interface {
	Ping() error
}

type SQLStorage struct {
	DB *sql.DB
}

type URLPair struct {
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
}

// User представляет пользователя в системе
type User struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

// Order представляет заказ в системе
type Order struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	OrderNum  string    `json:"order_num"`
	Status    string    `json:"status"`
	Accrual   *float64  `json:"accrual,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Withdrawal представляет вывод средств пользователя
type Withdrawal struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	OrderNum    string    `json:"order"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// Balance представляет баланс пользователя
type Balance struct {
	UserID    int     `json:"user_id"`
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// NewSQLStorage создаёт SQLStorage и инициализирует таблицу
func NewSQLStorage(db *sql.DB) (*SQLStorage, error) {
	storage := &SQLStorage{DB: db}
	if err := storage.initTable(); err != nil {
		return nil, err
	}
	return storage, nil
}

func (s *SQLStorage) initTable() error {
	// Создаем таблицу пользователей
	_, err := s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			login VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		logger.Log.Error("Failed to create users table", zap.Error(err))
		return err
	} else {
		logger.Log.Info("Table users created or already exists")
	}

	// Создаем таблицу заказов
	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_num VARCHAR(255) UNIQUE NOT NULL,
			status VARCHAR(50) NOT NULL DEFAULT 'NEW',
			accrual DECIMAL(10,2) DEFAULT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		logger.Log.Error("Failed to create orders table", zap.Error(err))
		return err
	} else {
		logger.Log.Info("Table orders created or already exists")
	}

	// Создаем таблицу выводов средств
	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS withdrawals (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_num VARCHAR(255) NOT NULL,
			sum DECIMAL(10,2) NOT NULL,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		logger.Log.Error("Failed to create withdrawals table", zap.Error(err))
		return err
	} else {
		logger.Log.Info("Table withdrawals created or already exists")
	}

	// Создаем таблицу балансов
	_, err = s.DB.Exec(`
		CREATE TABLE IF NOT EXISTS balances (
			user_id INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
			current DECIMAL(10,2) DEFAULT 0,
			withdrawn DECIMAL(10,2) DEFAULT 0,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		logger.Log.Error("Failed to create balances table", zap.Error(err))
		return err
	} else {
		logger.Log.Info("Table balances created or already exists")
	}

	return nil
}

// Ping проверяет соединение с базой данных
func (s *SQLStorage) Ping(ctx context.Context) error {
	return s.DB.PingContext(ctx)
}

// Close закрывает соединение с базой данных
func (s *SQLStorage) Close() error {
	return s.DB.Close()
}

// CreateUser создает нового пользователя
func (s *SQLStorage) CreateUser(ctx context.Context, login, password string) error {
	query := `INSERT INTO users (login, password) VALUES ($1, $2)`
	_, err := s.DB.ExecContext(ctx, query, login, password)
	if err != nil {
		logger.Log.Error("Failed to create user", zap.Error(err))
		return fmt.Errorf("failed to create user: %w", err)
	}
	return nil
}

// GetUserByLogin получает пользователя по логину
func (s *SQLStorage) GetUserByLogin(ctx context.Context, login string) (*User, error) {
	var user User
	query := `SELECT id, login, password FROM users WHERE login = $1`

	err := s.DB.QueryRowContext(ctx, query, login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		logger.Log.Error("Failed to get user by login", zap.Error(err))
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	return &user, nil
}

// UserExists проверяет, существует ли пользователь с таким логином
func (s *SQLStorage) UserExists(ctx context.Context, login string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)`

	err := s.DB.QueryRowContext(ctx, query, login).Scan(&exists)
	if err != nil {
		logger.Log.Error("Failed to check if user exists", zap.Error(err))
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}
	return exists, nil
}

// CreateOrder создает новый заказ
func (s *SQLStorage) CreateOrder(ctx context.Context, userID int, orderNum string) (*Order, error) {
	var order Order
	query := `INSERT INTO orders (user_id, order_num, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $4) RETURNING id, user_id, order_num, status, accrual, created_at, updated_at`

	now := time.Now()
	err := s.DB.QueryRowContext(ctx, query, userID, orderNum, "NEW", now).Scan(
		&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		logger.Log.Error("Failed to create order", zap.Error(err))
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return &order, nil
}

// GetOrderByNumber получает заказ по номеру
func (s *SQLStorage) GetOrderByNumber(ctx context.Context, orderNum string) (*Order, error) {
	var order Order
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE order_num = $1`

	err := s.DB.QueryRowContext(ctx, query, orderNum).Scan(
		&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)

	if err == sql.ErrNoRows {
		logger.Log.Info("Order not found", zap.String("orderNum", orderNum))
		return nil, nil
	}
	if err != nil {
		logger.Log.Error("Failed to get order by number", zap.Error(err))
		return nil, fmt.Errorf("failed to get order by number: %w", err)
	}

	logger.Log.Info("Order found", zap.String("orderNum", orderNum), zap.Int("userID", order.UserID))
	return &order, nil
}

// GetOrdersByUser получает все заказы пользователя
func (s *SQLStorage) GetOrdersByUser(ctx context.Context, userID int) ([]*Order, error) {
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := s.DB.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error("Failed to query user orders", zap.Error(err))
		return nil, fmt.Errorf("failed to get orders by user: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		if err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt); err != nil {
			logger.Log.Error("Failed to scan order row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating order rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating order rows: %w", err)
	}

	return orders, nil
}

// UpdateOrderStatus обновляет статус заказа
func (s *SQLStorage) UpdateOrderStatus(ctx context.Context, orderID int, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`

	_, err := s.DB.ExecContext(ctx, query, status, orderID)
	if err != nil {
		logger.Log.Error("Failed to update order status", zap.Error(err))
		return fmt.Errorf("failed to update order status: %w", err)
	}
	return nil
}

// UpdateOrderStatusAndAccrual обновляет статус заказа и начисление
func (s *SQLStorage) UpdateOrderStatusAndAccrual(ctx context.Context, orderNum string, status string, accrual *float64) error {
	query := `UPDATE orders SET status = $1, accrual = $2, updated_at = CURRENT_TIMESTAMP WHERE order_num = $3`

	_, err := s.DB.ExecContext(ctx, query, status, accrual, orderNum)
	if err != nil {
		logger.Log.Error("Failed to update order status and accrual", zap.Error(err))
		return fmt.Errorf("failed to update order status and accrual: %w", err)
	}
	return nil
}

// GetUserBalance получает текущий баланс и сумму использованных баллов пользователя
func (s *SQLStorage) GetUserBalance(ctx context.Context, userID int) (float64, float64, error) {
	var current, withdrawn float64

	// Получаем текущий баланс (сумма всех начислений)
	err := s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(accrual), 0) 
		FROM orders 
		WHERE user_id = $1 AND accrual IS NOT NULL AND accrual > 0
	`, userID).Scan(&current)
	if err != nil {
		logger.Log.Error("Failed to get current balance", zap.Error(err))
		return 0, 0, fmt.Errorf("failed to get current balance: %w", err)
	}

	// Получаем сумму использованных баллов (сумма всех списаний)
	err = s.DB.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(ABS(accrual)), 0) 
		FROM orders 
		WHERE user_id = $1 AND accrual IS NOT NULL AND accrual < 0
	`, userID).Scan(&withdrawn)
	if err != nil {
		logger.Log.Error("Failed to get withdrawn balance", zap.Error(err))
		return 0, 0, fmt.Errorf("failed to get withdrawn balance: %w", err)
	}

	return current, withdrawn, nil
}

// UpdateOrderAccrual обновляет начисление заказа
func (s *SQLStorage) UpdateOrderAccrual(ctx context.Context, orderNum string, accrual float64) error {
	query := `UPDATE orders SET accrual = $1, updated_at = CURRENT_TIMESTAMP WHERE order_num = $2`

	_, err := s.DB.ExecContext(ctx, query, accrual, orderNum)
	if err != nil {
		logger.Log.Error("Failed to update order accrual", zap.Error(err))
		return fmt.Errorf("failed to update order accrual: %w", err)
	}
	return nil
}

// CreateWithdrawal создает вывод средств
func (s *SQLStorage) CreateWithdrawal(ctx context.Context, userID int, orderNum string, sum float64) error {
	query := `INSERT INTO withdrawals (user_id, order_num, sum) VALUES ($1, $2, $3)`

	_, err := s.DB.ExecContext(ctx, query, userID, orderNum, sum)
	if err != nil {
		logger.Log.Error("Failed to create withdrawal", zap.Error(err))
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}
	return nil
}

// GetUserWithdrawals получает все выводы средств пользователя
func (s *SQLStorage) GetUserWithdrawals(ctx context.Context, userID int) ([]*Withdrawal, error) {
	query := `SELECT id, user_id, order_num, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`

	rows, err := s.DB.QueryContext(ctx, query, userID)
	if err != nil {
		logger.Log.Error("Failed to query user withdrawals", zap.Error(err))
		return nil, fmt.Errorf("failed to get withdrawals by user: %w", err)
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		withdrawal := &Withdrawal{}
		if err := rows.Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.OrderNum, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			logger.Log.Error("Failed to scan withdrawal row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating withdrawal rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating withdrawal rows: %w", err)
	}

	return withdrawals, nil
}

// GetOrdersByStatus получает заказы по статусам
func (s *SQLStorage) GetOrdersByStatus(ctx context.Context, statuses []string) ([]*Order, error) {
	if len(statuses) == 0 {
		return []*Order{}, nil
	}

	// Строим запрос с параметрами для статусов
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE status = ANY($1) ORDER BY created_at ASC`

	rows, err := s.DB.QueryContext(ctx, query, statuses)
	if err != nil {
		logger.Log.Error("Failed to get orders by status", zap.Error(err))
		return nil, fmt.Errorf("failed to get orders by status: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			logger.Log.Error("Failed to scan order row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating order rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating order rows: %w", err)
	}

	return orders, nil
}

// GetOrdersByStatusPaginated получает заказы по статусам с пагинацией
func (s *SQLStorage) GetOrdersByStatusPaginated(ctx context.Context, statuses []string, limit, offset int) ([]*Order, error) {
	if len(statuses) == 0 {
		return []*Order{}, nil
	}

	// Строим запрос с параметрами для статусов и пагинацией
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE status = ANY($1) ORDER BY created_at ASC LIMIT $2 OFFSET $3`

	rows, err := s.DB.QueryContext(ctx, query, statuses, limit, offset)
	if err != nil {
		logger.Log.Error("Failed to get orders by status with pagination", zap.Error(err))
		return nil, fmt.Errorf("failed to get orders by status with pagination: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			logger.Log.Error("Failed to scan order row", zap.Error(err))
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating order rows", zap.Error(err))
		return nil, fmt.Errorf("error iterating order rows: %w", err)
	}

	return orders, nil
}

// UpdateOrderStatusAndBalance атомарно обновляет статус заказа и баланс пользователя
func (s *SQLStorage) UpdateOrderStatusAndBalance(ctx context.Context, orderNumber string, status string, accrual *float64, userID int, newCurrent, withdrawn float64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Обновляем статус заказа
	orderQuery := `UPDATE orders SET status = $1, accrual = $2, updated_at = CURRENT_TIMESTAMP WHERE order_num = $3`
	_, err = tx.ExecContext(ctx, orderQuery, status, accrual, orderNumber)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Обновляем баланс
	balanceQuery := `INSERT INTO balances (user_id, current, withdrawn, updated_at) VALUES ($1, $2, $3, CURRENT_TIMESTAMP)
		ON CONFLICT (user_id) DO UPDATE SET current = $2, withdrawn = $3, updated_at = CURRENT_TIMESTAMP`
	_, err = tx.ExecContext(ctx, balanceQuery, userID, newCurrent, withdrawn)
	if err != nil {
		return fmt.Errorf("failed to update balance: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
