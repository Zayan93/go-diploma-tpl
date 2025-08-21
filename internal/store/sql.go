package store

import (
	"database/sql"
	"fmt"
	"strings"

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
	ID        int      `json:"id"`
	UserID    int      `json:"user_id"`
	OrderNum  string   `json:"order_num"`
	Status    string   `json:"status"`
	Accrual   *float64 `json:"accrual,omitempty"`
	CreatedAt string   `json:"created_at"`
	UpdatedAt string   `json:"updated_at"`
}

// Withdrawal представляет вывод средств пользователя
type Withdrawal struct {
	ID          int     `json:"id"`
	UserID      int     `json:"user_id"`
	OrderNum    string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
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
			order_num VARCHAR(255) UNIQUE NOT NULL,
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

	return nil
}

// Store сохраняет сокращённый URL
func (s *SQLStorage) Store(id, url string, userID string) error {
	logger.Log.Info("SQL storage store")
	_, err := s.DB.Exec(`INSERT INTO short_urls (short_id, original_url, user_id) VALUES ($1, $2, $3) ON CONFLICT (short_id) DO NOTHING`, id, url, userID)
	if err != nil {
		logger.Log.Error("Failed to store URL in SQL", zap.Error(err))
	}
	return err
}

// Get возвращает оригинальный URL по short_id
func (s *SQLStorage) Get(id string) (string, bool) {
	var url string
	var isDeleted bool
	err := s.DB.QueryRow(`SELECT original_url, is_deleted FROM short_urls WHERE short_id = $1`, id).Scan(&url, &isDeleted)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		return "", false
	}
	// Если URL помечен как удаленный, возвращаем false
	if isDeleted {
		return "", false
	}
	return url, true
}

// GetShortIDByOriginalURL returns the short_id for a given original_url, if it exists.
func (s *SQLStorage) GetShortIDByOriginalURL(url string) (string, bool) {
	var shortID string
	err := s.DB.QueryRow(`SELECT short_id FROM short_urls WHERE original_url = $1`, url).Scan(&shortID)
	if err == sql.ErrNoRows {
		return "", false
	}
	if err != nil {
		return "", false
	}
	return shortID, true
}

// Ping проверяет соединение с базой данных
func (s *SQLStorage) Ping() error {
	return s.DB.Ping()
}

// StoreBatch сохраняет множество сокращённых URL в рамках одной транзакции
func (s *SQLStorage) StoreBatch(pairs map[string]string, userID string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO short_urls (short_id, original_url, user_id) VALUES ($1, $2, $3) ON CONFLICT (short_id) DO NOTHING`)
	if err != nil {
		tx.Rollback()
		return err
	}
	defer stmt.Close()
	for id, url := range pairs {
		if _, err := stmt.Exec(id, url, userID); err != nil {
			tx.Rollback()
			return err
		}
	}
	return tx.Commit()
}

func (s *SQLStorage) GetURLsByUser(userID string) ([]URLPair, error) {
	rows, err := s.DB.Query(`SELECT short_id, original_url FROM short_urls WHERE user_id = $1 AND is_deleted = FALSE ORDER BY created_at DESC`, userID)
	if err != nil {
		logger.Log.Error("Failed to query user URLs", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var pairs []URLPair
	for rows.Next() {
		var shortID, originalURL string
		if err := rows.Scan(&shortID, &originalURL); err != nil {
			logger.Log.Error("Failed to scan row", zap.Error(err))
			return nil, err
		}
		pairs = append(pairs, URLPair{
			ShortURL:    shortID,
			OriginalURL: originalURL,
		})
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating rows", zap.Error(err))
		return nil, err
	}

	logger.Log.Info("Found URLs for user in SQL storage", zap.String("userID", userID), zap.Int("count", len(pairs)))
	return pairs, nil
}

// DeleteURLs помечает URL как удаленные для указанного пользователя
func (s *SQLStorage) DeleteURLs(shortIDs []string, userID string) error {
	if len(shortIDs) == 0 {
		return nil
	}

	// Создаем плейсхолдеры для IN запроса
	placeholders := make([]string, len(shortIDs))
	args := make([]interface{}, len(shortIDs)+1)

	for i, id := range shortIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}
	args[len(shortIDs)] = userID

	query := fmt.Sprintf(`
		UPDATE short_urls 
		SET is_deleted = TRUE 
		WHERE short_id IN (%s) AND user_id = $%d
	`, strings.Join(placeholders, ","), len(shortIDs)+1)

	_, err := s.DB.Exec(query, args...)
	if err != nil {
		logger.Log.Error("Failed to delete URLs", zap.Error(err))
		return err
	}

	logger.Log.Info("Successfully marked URLs as deleted", zap.Strings("shortIDs", shortIDs), zap.String("userID", userID))
	return nil
}

// GetWithDeletedFlag возвращает URL с информацией о том, удален ли он
func (s *SQLStorage) GetWithDeletedFlag(id string) (string, bool, bool) {
	var url string
	var isDeleted bool

	err := s.DB.QueryRow(`
		SELECT original_url, is_deleted 
		FROM short_urls 
		WHERE short_id = $1
	`, id).Scan(&url, &isDeleted)

	if err == sql.ErrNoRows {
		return "", false, false
	}
	if err != nil {
		logger.Log.Error("Failed to get URL with deleted flag", zap.Error(err))
		return "", false, false
	}

	return url, true, isDeleted
}

// CreateUser создает нового пользователя
func (s *SQLStorage) CreateUser(login, password string) error {
	_, err := s.DB.Exec(`INSERT INTO users (login, password) VALUES ($1, $2)`, login, password)
	if err != nil {
		logger.Log.Error("Failed to create user", zap.Error(err))
	}
	return err
}

// GetUserByLogin возвращает пользователя по логину
func (s *SQLStorage) GetUserByLogin(login string) (*User, error) {
	user := &User{}
	err := s.DB.QueryRow(`SELECT id, login, password FROM users WHERE login = $1`, login).Scan(&user.ID, &user.Login, &user.Password)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		logger.Log.Error("Failed to get user by login", zap.Error(err))
		return nil, err
	}
	return user, nil
}

// UserExists проверяет, существует ли пользователь с данным логином
func (s *SQLStorage) UserExists(login string) (bool, error) {
	var exists bool
	err := s.DB.QueryRow(`SELECT EXISTS(SELECT 1 FROM users WHERE login = $1)`, login).Scan(&exists)
	if err != nil {
		logger.Log.Error("Failed to check if user exists", zap.Error(err))
		return false, err
	}
	return exists, nil
}

// CreateOrder создает новый заказ и возвращает созданный заказ
func (s *SQLStorage) CreateOrder(userID int, orderNum string) (*Order, error) {
	var order Order
	query := `INSERT INTO orders (user_id, order_num, status, created_at, updated_at) VALUES ($1, $2, $3, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP) RETURNING id, user_id, order_num, status, accrual, created_at, updated_at`

	err := s.DB.QueryRow(query, userID, orderNum, "NEW").Scan(
		&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)
	if err != nil {
		logger.Log.Error("Failed to create order", zap.Error(err))
		return nil, err
	}

	// Вычисляем начисление баллов на основе номера заказа
	accrual := s.calculateOrderAccrual(orderNum)
	if accrual > 0 {
		// Обновляем заказ с начислением
		err = s.UpdateOrderAccrual(orderNum, accrual)
		if err != nil {
			logger.Log.Error("Failed to update order accrual", zap.Error(err))
		}
		// Обновляем статус заказа
		err = s.UpdateOrderStatus(order.ID, "PROCESSED")
		if err != nil {
			logger.Log.Error("Failed to update order status", zap.Error(err))
		}
	}

	return &order, nil
}

// calculateOrderAccrual вычисляет начисление баллов на основе номера заказа
func (s *SQLStorage) calculateOrderAccrual(orderNum string) float64 {
	// Простая логика начисления: если номер заказа заканчивается на 0, 1, 2, 3, 4, 5, 6, 7, 8, 9
	// то начисляем соответствующее количество баллов (0-9)
	if len(orderNum) == 0 {
		return 0
	}

	lastDigit := orderNum[len(orderNum)-1]
	digit := int(lastDigit - '0')

	// Начисляем от 0 до 9 баллов в зависимости от последней цифры
	// Для заказов, заканчивающихся на 0 - 0 баллов
	// Для заказов, заканчивающихся на 1 - 1 балл
	// И так далее...

	// Можно сделать более сложную логику, например:
	// - Если номер заказа делится на 3 без остатка - 100 баллов
	// - Если номер заказа делится на 5 без остатка - 50 баллов
	// - Иначе - 10 баллов

	if len(orderNum)%3 == 0 {
		return 100.0
	} else if len(orderNum)%5 == 0 {
		return 50.0
	} else {
		return float64(digit) * 10.0
	}
}

// GetOrderByNumber возвращает заказ по номеру
func (s *SQLStorage) GetOrderByNumber(orderNum string) (*Order, error) {
	var order Order
	err := s.DB.QueryRow(`
		SELECT id, user_id, order_num, status, accrual, created_at, updated_at 
		FROM orders 
		WHERE order_num = $1
	`, orderNum).Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt)

	if err == sql.ErrNoRows {
		logger.Log.Info("Order not found", zap.String("orderNum", orderNum))
		return nil, nil
	}
	if err != nil {
		logger.Log.Error("Failed to get order by number", zap.Error(err))
		return nil, err
	}

	logger.Log.Info("Order found", zap.String("orderNum", orderNum), zap.Int("userID", order.UserID))
	return &order, nil
}

// GetOrdersByUser возвращает все заказы пользователя
func (s *SQLStorage) GetOrdersByUser(userID int) ([]*Order, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, order_num, status, accrual, created_at, updated_at 
		FROM orders 
		WHERE user_id = $1 
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		logger.Log.Error("Failed to query user orders", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		order := &Order{}
		if err := rows.Scan(&order.ID, &order.UserID, &order.OrderNum, &order.Status, &order.Accrual, &order.CreatedAt, &order.UpdatedAt); err != nil {
			logger.Log.Error("Failed to scan order row", zap.Error(err))
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating order rows", zap.Error(err))
		return nil, err
	}

	return orders, nil
}

// UpdateOrderStatus обновляет статус заказа
func (s *SQLStorage) UpdateOrderStatus(orderID int, status string) error {
	_, err := s.DB.Exec(`
		UPDATE orders 
		SET status = $1, updated_at = CURRENT_TIMESTAMP 
		WHERE id = $2
	`, status, orderID)
	if err != nil {
		logger.Log.Error("Failed to update order status", zap.Error(err))
	}
	return err
}

// GetUserBalance возвращает текущий баланс и сумму использованных баллов пользователя
func (s *SQLStorage) GetUserBalance(userID int) (float64, float64, error) {
	var current, withdrawn float64

	// Получаем текущий баланс (сумма всех начислений)
	err := s.DB.QueryRow(`
		SELECT COALESCE(SUM(accrual), 0) 
		FROM orders 
		WHERE user_id = $1 AND accrual IS NOT NULL AND accrual > 0
	`, userID).Scan(&current)
	if err != nil {
		logger.Log.Error("Failed to get current balance", zap.Error(err))
		return 0, 0, err
	}

	// Получаем сумму использованных баллов (сумма всех списаний)
	err = s.DB.QueryRow(`
		SELECT COALESCE(SUM(ABS(accrual)), 0) 
		FROM orders 
		WHERE user_id = $1 AND accrual IS NOT NULL AND accrual < 0
	`, userID).Scan(&withdrawn)
	if err != nil {
		logger.Log.Error("Failed to get withdrawn balance", zap.Error(err))
		return 0, 0, err
	}

	return current, withdrawn, nil
}

// UpdateOrderAccrual обновляет начисление баллов для заказа
func (s *SQLStorage) UpdateOrderAccrual(orderNum string, accrual float64) error {
	_, err := s.DB.Exec(`
		UPDATE orders 
		SET accrual = $1, updated_at = CURRENT_TIMESTAMP 
		WHERE order_num = $2
	`, accrual, orderNum)
	if err != nil {
		logger.Log.Error("Failed to update order accrual", zap.Error(err))
	}
	return err
}

// CreateWithdrawal создает новый вывод средств
func (s *SQLStorage) CreateWithdrawal(userID int, orderNum string, sum float64) error {
	_, err := s.DB.Exec(`
		INSERT INTO withdrawals (user_id, order_num, sum) 
		VALUES ($1, $2, $3)
	`, userID, orderNum, sum)
	if err != nil {
		logger.Log.Error("Failed to create withdrawal", zap.Error(err))
	}
	return err
}

// GetUserWithdrawals возвращает все выводы средств пользователя
func (s *SQLStorage) GetUserWithdrawals(userID int) ([]*Withdrawal, error) {
	rows, err := s.DB.Query(`
		SELECT id, user_id, order_num, sum, processed_at 
		FROM withdrawals 
		WHERE user_id = $1 
		ORDER BY processed_at DESC
	`, userID)
	if err != nil {
		logger.Log.Error("Failed to query user withdrawals", zap.Error(err))
		return nil, err
	}
	defer rows.Close()

	var withdrawals []*Withdrawal
	for rows.Next() {
		withdrawal := &Withdrawal{}
		if err := rows.Scan(&withdrawal.ID, &withdrawal.UserID, &withdrawal.OrderNum, &withdrawal.Sum, &withdrawal.ProcessedAt); err != nil {
			logger.Log.Error("Failed to scan withdrawal row", zap.Error(err))
			return nil, err
		}
		withdrawals = append(withdrawals, withdrawal)
	}

	if err = rows.Err(); err != nil {
		logger.Log.Error("Error iterating withdrawal rows", zap.Error(err))
		return nil, err
	}

	return withdrawals, nil
}
