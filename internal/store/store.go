package store

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
)

// UserStorage интерфейс для работы с пользователями
type UserStorage interface {
	CreateUser(ctx context.Context, login, password string) error
	GetUserByLogin(ctx context.Context, login string) (*User, error)
	UserExists(ctx context.Context, login string) (bool, error)
}

// OrderStorage интерфейс для работы с заказами
type OrderStorage interface {
	CreateOrder(ctx context.Context, userID int, orderNum string) error
	GetOrderByNumber(ctx context.Context, orderNum string) (*Order, error)
	GetOrdersByUser(ctx context.Context, userID int) ([]*Order, error)
	UpdateOrderStatus(ctx context.Context, orderID int, status string) error
	GetUserBalance(ctx context.Context, userID int) (float64, float64, error)
	UpdateOrderAccrual(ctx context.Context, orderNum string, accrual float64) error
	CreateWithdrawal(ctx context.Context, userID int, orderNum string, sum float64) error
	GetUserWithdrawals(ctx context.Context, userID int) ([]*Withdrawal, error)
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
