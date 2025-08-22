package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Zayan93/go-diploma-tpl/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseStorage реализация хранилища на PostgreSQL
type DatabaseStorage struct {
	pool *pgxpool.Pool
}

// NewDatabaseStorage создает новое подключение к базе данных
func NewDatabaseStorage(ctx context.Context, databaseURI string) (*DatabaseStorage, error) {
	pool, err := pgxpool.New(ctx, databaseURI)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Проверяем соединение
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	// Инициализируем таблицы
	storage := &DatabaseStorage{pool: pool}
	if err := storage.initTable(ctx); err != nil {
		return nil, fmt.Errorf("failed to initialize tables: %w", err)
	}

	return storage, nil
}

// Close закрывает соединение с базой данных
func (s *DatabaseStorage) Close() error {
	s.pool.Close()
	return nil
}

// Ping проверяет соединение с базой данных
func (s *DatabaseStorage) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// initTable инициализирует таблицы в базе данных
func (s *DatabaseStorage) initTable(ctx context.Context) error {
	// Создаем таблицу пользователей
	_, err := s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			login VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create users table: %w", err)
	}

	// Создаем таблицу заказов
	_, err = s.pool.Exec(ctx, `
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
		return fmt.Errorf("failed to create orders table: %w", err)
	}

	// Создаем таблицу выводов средств
	_, err = s.pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS withdrawals (
			id SERIAL PRIMARY KEY,
			user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			order_num VARCHAR(255) NOT NULL,
			sum DECIMAL(10,2) NOT NULL,
			processed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	if err != nil {
		return fmt.Errorf("failed to create withdrawals table: %w", err)
	}

	return nil
}

// CreateUser создает нового пользователя
func (s *DatabaseStorage) CreateUser(ctx context.Context, login, password string) error {
	query := `INSERT INTO users (login, password) VALUES ($1, $2)`

	_, err := s.pool.Exec(ctx, query, login, password)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	return nil
}

// GetUserByLogin получает пользователя по логину
func (s *DatabaseStorage) GetUserByLogin(ctx context.Context, login string) (*store.User, error) {
	var user store.User
	query := `SELECT id, login, password FROM users WHERE login = $1`

	err := s.pool.QueryRow(ctx, query, login).Scan(&user.ID, &user.Login, &user.Password)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by login: %w", err)
	}

	return &user, nil
}

// UserExists проверяет, существует ли пользователь с таким логином
func (s *DatabaseStorage) UserExists(ctx context.Context, login string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)`

	err := s.pool.QueryRow(ctx, query, login).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check if user exists: %w", err)
	}
	return exists, nil
}

// CreateOrder создает новый заказ
func (s *DatabaseStorage) CreateOrder(ctx context.Context, userID int, orderNum string) (*store.Order, error) {
	var order store.Order
	query := `INSERT INTO orders (user_id, order_num, status, created_at, updated_at) VALUES ($1, $2, $3, $4, $4) RETURNING id, user_id, order_num, status, accrual, created_at, updated_at`

	now := time.Now()
	err := s.pool.QueryRow(ctx, query, userID, orderNum, "NEW", now).Scan(
		&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("failed to create order: %w", err)
	}

	return &order, nil
}

// GetOrderByNumber получает заказ по номеру
func (s *DatabaseStorage) GetOrderByNumber(ctx context.Context, orderNum string) (*store.Order, error) {
	var order store.Order
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE order_num = $1`

	err := s.pool.QueryRow(ctx, query, orderNum).Scan(
		&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get order by number: %w", err)
	}

	return &order, nil
}

// GetOrdersByUser получает все заказы пользователя
func (s *DatabaseStorage) GetOrdersByUser(ctx context.Context, userID int) ([]*store.Order, error) {
	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE user_id = $1 ORDER BY created_at DESC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user orders: %w", err)
	}
	defer rows.Close()

	var orders []*store.Order
	for rows.Next() {
		var order store.Order
		err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}

	return orders, nil
}

// UpdateOrderStatus обновляет статус заказа
func (s *DatabaseStorage) UpdateOrderStatus(ctx context.Context, orderID int, status string) error {
	query := `UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3`

	now := time.Now()
	_, err := s.pool.Exec(ctx, query, status, now, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	return nil
}

// UpdateOrderStatusAndAccrual обновляет статус и начисление заказа
func (s *DatabaseStorage) UpdateOrderStatusAndAccrual(ctx context.Context, orderNum string, status string, accrual *float64) error {
	query := `UPDATE orders SET status = $1, accrual = $2, updated_at = $3 WHERE order_num = $4`

	now := time.Now()
	_, err := s.pool.Exec(ctx, query, status, accrual, now, orderNum)
	if err != nil {
		return fmt.Errorf("failed to update order status and accrual: %w", err)
	}

	return nil
}

// GetUserBalance получает баланс пользователя
func (s *DatabaseStorage) GetUserBalance(ctx context.Context, userID int) (float64, float64, error) {
	// Получаем текущий баланс из заказов
	var current float64
	currentQuery := `SELECT COALESCE(SUM(accrual), 0) FROM orders WHERE user_id = $1 AND status = 'PROCESSED' AND accrual IS NOT NULL`

	err := s.pool.QueryRow(ctx, currentQuery, userID).Scan(&current)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get current balance: %w", err)
	}

	// Получаем сумму выводов
	var withdrawn float64
	withdrawnQuery := `SELECT COALESCE(SUM(sum), 0) FROM withdrawals WHERE user_id = $1`

	err = s.pool.QueryRow(ctx, withdrawnQuery, userID).Scan(&withdrawn)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to get withdrawn amount: %w", err)
	}

	return current, withdrawn, nil
}

// UpdateBalance обновляет баланс пользователя
func (s *DatabaseStorage) UpdateBalance(ctx context.Context, userID int, current, withdrawn float64) error {
	// В данной реализации баланс вычисляется из заказов и выводов
	// Этот метод может использоваться для будущих расширений
	return nil
}

// UpdateOrderAccrual обновляет начисление заказа
func (s *DatabaseStorage) UpdateOrderAccrual(ctx context.Context, orderNum string, accrual float64) error {
	query := `UPDATE orders SET accrual = $1, updated_at = $2 WHERE order_num = $3`

	now := time.Now()
	_, err := s.pool.Exec(ctx, query, accrual, now, orderNum)
	if err != nil {
		return fmt.Errorf("failed to update order accrual: %w", err)
	}

	return nil
}

// CreateWithdrawal создает вывод средств
func (s *DatabaseStorage) CreateWithdrawal(ctx context.Context, userID int, orderNum string, sum float64) error {
	query := `INSERT INTO withdrawals (user_id, order_num, sum, processed_at) VALUES ($1, $2, $3, $4)`

	now := time.Now()
	_, err := s.pool.Exec(ctx, query, userID, orderNum, sum, now)
	if err != nil {
		return fmt.Errorf("failed to create withdrawal: %w", err)
	}

	return nil
}

// GetUserWithdrawals получает выводы средств пользователя
func (s *DatabaseStorage) GetUserWithdrawals(ctx context.Context, userID int) ([]*store.Withdrawal, error) {
	query := `SELECT id, user_id, order_num, sum, processed_at FROM withdrawals WHERE user_id = $1 ORDER BY processed_at DESC`

	rows, err := s.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to query user withdrawals: %w", err)
	}
	defer rows.Close()

	var withdrawals []*store.Withdrawal
	for rows.Next() {
		var withdrawal store.Withdrawal
		err := rows.Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.OrderNum, &withdrawal.Sum, &withdrawal.ProcessedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan withdrawal: %w", err)
		}
		withdrawals = append(withdrawals, &withdrawal)
	}

	return withdrawals, nil
}

// GetOrdersByStatus получает заказы по статусам
func (s *DatabaseStorage) GetOrdersByStatus(ctx context.Context, statuses []string) ([]*store.Order, error) {
	if len(statuses) == 0 {
		return []*store.Order{}, nil
	}

	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE status = ANY($1) ORDER BY created_at ASC`

	rows, err := s.pool.Query(ctx, query, statuses)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by status: %w", err)
	}
	defer rows.Close()

	var orders []*store.Order
	for rows.Next() {
		var order store.Order
		err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}

	return orders, nil
}

// GetOrdersByStatusPaginated получает заказы по статусам с пагинацией
func (s *DatabaseStorage) GetOrdersByStatusPaginated(ctx context.Context, statuses []string, limit, offset int) ([]*store.Order, error) {
	if len(statuses) == 0 {
		return []*store.Order{}, nil
	}

	query := `SELECT id, user_id, order_num, status, accrual, created_at, updated_at FROM orders WHERE status = ANY($1) ORDER BY created_at ASC LIMIT $2 OFFSET $3`

	rows, err := s.pool.Query(ctx, query, statuses, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get orders by status with pagination: %w", err)
	}
	defer rows.Close()

	var orders []*store.Order
	for rows.Next() {
		var order store.Order
		err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}

	return orders, nil
}

// UpdateOrderStatusAndBalance атомарно обновляет статус заказа и баланс пользователя
func (s *DatabaseStorage) UpdateOrderStatusAndBalance(ctx context.Context, orderNumber string, status string, accrual *float64, userID int, newCurrent, withdrawn float64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Обновляем статус заказа
	orderQuery := `UPDATE orders SET status = $1, accrual = $2, updated_at = $3 WHERE order_num = $4`
	now := time.Now()
	_, err = tx.Exec(ctx, orderQuery, status, accrual, now, orderNumber)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// В данной реализации баланс вычисляется из заказов и выводов
	// Поэтому просто обновляем заказ

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
