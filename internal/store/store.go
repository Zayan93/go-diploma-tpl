package store

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"time"
)

// UserStorage интерфейс для работы с пользователями
type UserStorage interface {
	CreateUser(ctx context.Context, login, password string) error
	GetUserByLogin(ctx context.Context, login string) (*User, error)
	UserExists(ctx context.Context, login string) (bool, error)
}

// OrderStorage интерфейс для работы с заказами
type OrderStorage interface {
	CreateOrder(ctx context.Context, userID int, orderNum string) (*Order, error)
	GetOrderByNumber(ctx context.Context, orderNum string) (*Order, error)
	GetOrdersByUser(ctx context.Context, userID int) ([]*Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int, status string) error
	UpdateOrderStatusAndAccrual(ctx context.Context, orderNum string, status string, accrual *float64) error
	// Методы для работы с балансами
	GetUserBalance(ctx context.Context, userID int) (float64, float64, error)
	UpdateBalance(ctx context.Context, userID int, current, withdrawn float64) error
	UpdateOrderAccrual(ctx context.Context, orderNum string, accrual float64) error
	CreateWithdrawal(ctx context.Context, userID int, orderNum string, sum float64) error
	GetUserWithdrawals(ctx context.Context, userID int) ([]*Withdrawal, error)
	GetOrdersByStatus(ctx context.Context, statuses []string) ([]*Order, error)
	GetOrdersByStatusPaginated(ctx context.Context, statuses []string, limit, offset int) ([]*Order, error)
	UpdateOrderStatusAndBalance(ctx context.Context, orderNumber string, status string, accrual *float64, userID int, newCurrent, withdrawn float64) error
}

// Storage общий интерфейс для всех операций с хранилищем
type Storage interface {
	UserStorage
	OrderStorage
	Ping(ctx context.Context) error
	Close() error
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

// Withdrawal представляет вывод средств
type Withdrawal struct {
	ID          int       `json:"id"`
	UserID      int       `json:"user_id"`
	OrderNum    string    `json:"order_num"`
	Sum         float64   `json:"sum"`
	ProcessedAt time.Time `json:"processed_at"`
}

// Balance представляет баланс пользователя
type Balance struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type Event struct {
	UUID        uint   `json:"uuid"`
	ShortURL    string `json:"short_url"`
	OriginalURL string `json:"original_url"`
	UserID      int    `json:"user_id"`
	DeletedFlag bool   `json:"deleted_flag"`
}

type Producer struct {
	file   *os.File // файл для записи
	writer *bufio.Writer
}

func NewProducer(filename string) (*Producer, error) {
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Producer{file: file, writer: bufio.NewWriter(file)}, nil
}

func (p *Producer) WriteEvent(event *Event) error {
	data, err := json.Marshal(&event)
	if err != nil {
		return err
	}
	if _, err := p.writer.Write(data); err != nil {
		return err
	}
	if err := p.writer.WriteByte('\n'); err != nil {
		return err
	}
	return p.writer.Flush()
}

func (p *Producer) Close() error {
	return p.file.Close()
}

type Consumer struct {
	file    *os.File
	scanner *bufio.Scanner
}

func NewConsumer(filename string) (*Consumer, error) {
	file, err := os.OpenFile(filename, os.O_RDONLY|os.O_CREATE, 0666)
	if err != nil {
		return nil, err
	}
	return &Consumer{file: file, scanner: bufio.NewScanner(file)}, nil
}

func (c *Consumer) Close() error {
	return c.file.Close()
}

func (c *Consumer) ReadEvent() (*Event, error) {
	if !c.scanner.Scan() {
		return nil, c.scanner.Err()
	}
	data := c.scanner.Bytes()
	event := Event{}
	err := json.Unmarshal(data, &event)
	if err != nil {
		return nil, err
	}
	return &event, nil
}
