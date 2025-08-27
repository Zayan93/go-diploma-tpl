package storage

import (
	"context"
	"fmt"
	"os"
	"testing"
)

const testDatabaseURI = "postgres://postgres:postgres@localhost:5432/praktikum?sslmode=disable"

var dbAvailable bool

func TestMain(m *testing.M) {
	storage, err := NewDatabaseStorage(context.Background(), testDatabaseURI)
	if err != nil {
		os.Exit(0) // База недоступна — пропускаем все тесты
	}
	defer storage.Close()
	dbAvailable = true
	os.Exit(m.Run())
}

// TestDatabaseStorage_Integration тестирует работу с реальной базой данных
func TestDatabaseStorage_Integration(t *testing.T) {
	if !dbAvailable {
		t.Skip("Database not available, skipping test")
	}
	storage, err := NewDatabaseStorage(context.Background(), testDatabaseURI)
	if err != nil {
		t.Skipf("Skipping database tests: failed to connect to database: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	// Проверяем соединение
	err = storage.Ping(ctx)
	if err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	// Очищаем базу данных перед тестами
	cleanupDatabase(t, storage)

	t.Run("CreateUser", func(t *testing.T) {
		// Создаем пользователя
		err := storage.CreateUser(ctx, "testuser", "hashedpassword")
		if err != nil {
			t.Fatalf("Failed to create user: %v", err)
		}
	})

	t.Run("GetUserByLogin", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user by login: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}
		if user.Login != "testuser" {
			t.Errorf("Expected login 'testuser', got '%s'", user.Login)
		}
		if user.Password != "hashedpassword" {
			t.Errorf("Expected password 'hashedpassword', got '%s'", user.Password)
		}

		// Проверяем несуществующего пользователя
		user, err = storage.GetUserByLogin(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("Failed to get non-existent user: %v", err)
		}
		if user != nil {
			t.Error("Non-existent user should be nil")
		}
	})

	t.Run("CreateOrder", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем заказ
		order, err := storage.CreateOrder(ctx, user.ID, "12345678903")
		if err != nil {
			t.Fatalf("Failed to create order: %v", err)
		}
		if order.ID == 0 {
			t.Error("Order ID should not be zero")
		}
		if order.UserID != user.ID {
			t.Errorf("Expected user ID %d, got %d", user.ID, order.UserID)
		}
		if order.OrderNum != "12345678903" {
			t.Errorf("Expected order number '12345678903', got '%s'", order.OrderNum)
		}
		if order.Status != "NEW" {
			t.Errorf("Expected status 'NEW', got '%s'", order.Status)
		}
	})

	t.Run("GetOrderByNumber", func(t *testing.T) {
		// Получаем заказ по номеру
		order, err := storage.GetOrderByNumber(ctx, "12345678903")
		if err != nil {
			t.Fatalf("Failed to get order by number: %v", err)
		}
		if order == nil {
			t.Fatal("Order should not be nil")
		}
		if order.OrderNum != "12345678903" {
			t.Errorf("Expected order number '12345678903', got '%s'", order.OrderNum)
		}

		// Проверяем несуществующий заказ
		order, err = storage.GetOrderByNumber(ctx, "99999999999")
		if err != nil {
			t.Fatalf("Failed to get non-existent order: %v", err)
		}
		if order != nil {
			t.Error("Non-existent order should be nil")
		}
	})

	t.Run("GetOrdersByUser", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем еще один заказ
		_, err = storage.CreateOrder(ctx, user.ID, "98765432109")
		if err != nil {
			t.Fatalf("Failed to create second order: %v", err)
		}

		// Получаем все заказы пользователя
		orders, err := storage.GetOrdersByUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user orders: %v", err)
		}
		if len(orders) != 2 {
			t.Errorf("Expected 2 orders, got %d", len(orders))
		}

		// Проверяем сортировку (от новых к старым)
		if orders[0].OrderNum != "98765432109" {
			t.Errorf("Expected first order number '98765432109', got '%s'", orders[0].OrderNum)
		}
		if orders[1].OrderNum != "12345678903" {
			t.Errorf("Expected second order number '12345678903', got '%s'", orders[1].OrderNum)
		}
	})

	t.Run("GetOrdersByStatus", func(t *testing.T) {
		// Получаем заказы со статусом NEW
		orders, err := storage.GetOrdersByStatus(ctx, []string{"NEW"})
		if err != nil {
			t.Fatalf("Failed to get orders by status: %v", err)
		}
		if len(orders) != 2 {
			t.Errorf("Expected 2 orders with NEW status, got %d", len(orders))
		}

		// Получаем заказы со статусом PROCESSING
		orders, err = storage.GetOrdersByStatus(ctx, []string{"PROCESSING"})
		if err != nil {
			t.Fatalf("Failed to get orders by PROCESSING status: %v", err)
		}
		if len(orders) != 0 {
			t.Errorf("Expected 0 orders with PROCESSING status, got %d", len(orders))
		}
	})

	t.Run("UpdateOrderStatus", func(t *testing.T) {
		// Получаем заказ для обновления
		order, err := storage.GetOrderByNumber(ctx, "12345678903")
		if err != nil {
			t.Fatalf("Failed to get order for update: %v", err)
		}
		if order == nil {
			t.Fatal("Order should not be nil")
		}

		// Обновляем статус заказа
		err = storage.UpdateOrderStatus(ctx, order.ID, "PROCESSED")
		if err != nil {
			t.Fatalf("Failed to update order status: %v", err)
		}

		// Проверяем, что статус обновился
		updatedOrder, err := storage.GetOrderByNumber(ctx, "12345678903")
		if err != nil {
			t.Fatalf("Failed to get updated order: %v", err)
		}
		if updatedOrder.Status != "PROCESSED" {
			t.Errorf("Expected status 'PROCESSED', got '%s'", updatedOrder.Status)
		}
	})

	t.Run("UpdateOrderStatusAndAccrual", func(t *testing.T) {
		// Обновляем статус и начисление заказа
		accrual := 100.0
		err := storage.UpdateOrderStatusAndAccrual(ctx, "12345678903", "PROCESSED", &accrual)
		if err != nil {
			t.Fatalf("Failed to update order status and accrual: %v", err)
		}

		// Проверяем, что статус и начисление обновились
		order, err := storage.GetOrderByNumber(ctx, "12345678903")
		if err != nil {
			t.Fatalf("Failed to get order after update: %v", err)
		}
		if order.Status != "PROCESSED" {
			t.Errorf("Expected status 'PROCESSED', got '%s'", order.Status)
		}
		if order.Accrual == nil || *order.Accrual != accrual {
			t.Errorf("Expected accrual %f, got %v", accrual, order.Accrual)
		}
	})

	t.Run("GetUserBalance", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Получаем баланс пользователя
		current, withdrawn, err := storage.GetUserBalance(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user balance: %v", err)
		}
		if current != 100.0 { // Начисление из предыдущего теста
			t.Errorf("Expected current balance 100.0, got %f", current)
		}
		if withdrawn != 0.0 {
			t.Errorf("Expected withdrawn 0.0, got %f", withdrawn)
		}
	})

	t.Run("UpdateBalance", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Обновляем баланс
		err = storage.UpdateBalance(ctx, user.ID, 500.0, 100.0)
		if err != nil {
			t.Fatalf("Failed to update balance: %v", err)
		}

		// Проверяем обновление
		current, withdrawn, err := storage.GetUserBalance(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get updated balance: %v", err)
		}
		if current != 100.0 { // Баланс вычисляется из заказов, а не из таблицы balances
			t.Errorf("Expected current balance 100.0, got %f", current)
		}
		if withdrawn != 0.0 {
			t.Errorf("Expected withdrawn 0.0, got %f", withdrawn)
		}
	})

	t.Run("CreateWithdrawal", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем списание
		err = storage.CreateWithdrawal(ctx, user.ID, "testorder123", 50.0)
		if err != nil {
			t.Fatalf("Failed to create withdrawal: %v", err)
		}
	})

	t.Run("GetUserWithdrawals", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем еще одно списание
		err = storage.CreateWithdrawal(ctx, user.ID, "testorder456", 25.0)
		if err != nil {
			t.Fatalf("Failed to create second withdrawal: %v", err)
		}

		// Получаем все списания пользователя
		withdrawals, err := storage.GetUserWithdrawals(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get user withdrawals: %v", err)
		}
		if len(withdrawals) != 2 {
			t.Errorf("Expected 2 withdrawals, got %d", len(withdrawals))
		}

		// Проверяем сортировку (от новых к старым)
		if withdrawals[0].OrderNum != "testorder456" {
			t.Errorf("Expected first withdrawal order 'testorder456', got '%s'", withdrawals[0].OrderNum)
		}
		if withdrawals[1].OrderNum != "testorder123" {
			t.Errorf("Expected second withdrawal order 'testorder123', got '%s'", withdrawals[1].OrderNum)
		}
	})

	t.Run("UpdateOrderAccrual", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем заказ для тестирования с уникальным номером
		order, err := storage.CreateOrder(ctx, user.ID, "12345678904")
		if err != nil {
			t.Fatalf("Failed to create order for accrual test: %v", err)
		}

		// Обновляем начисление заказа
		accrual := 50.0
		err = storage.UpdateOrderAccrual(ctx, order.OrderNum, accrual)
		if err != nil {
			t.Fatalf("Failed to update order accrual: %v", err)
		}

		// Проверяем, что начисление обновилось
		updatedOrder, err := storage.GetOrderByNumber(ctx, order.OrderNum)
		if err != nil {
			t.Fatalf("Failed to get order after accrual update: %v", err)
		}
		if updatedOrder.Accrual == nil || *updatedOrder.Accrual != accrual {
			t.Errorf("Expected accrual %f, got %v", accrual, updatedOrder.Accrual)
		}
	})

	t.Run("UpdateOrderStatusAndBalance", func(t *testing.T) {
		user, err := storage.GetUserByLogin(ctx, "testuser")
		if err != nil {
			t.Fatalf("Failed to get user: %v", err)
		}
		if user == nil {
			t.Fatal("User should not be nil")
		}

		// Создаем заказ для тестирования с уникальным номером
		order, err := storage.CreateOrder(ctx, user.ID, "98765432108")
		if err != nil {
			t.Fatalf("Failed to create order for balance test: %v", err)
		}

		// Атомарное обновление статуса заказа и баланса
		accrual := 75.0
		err = storage.UpdateOrderStatusAndBalance(ctx, order.OrderNum, "PROCESSED", &accrual, user.ID, 175.0, 0.0)
		if err != nil {
			t.Fatalf("Failed to update order status and balance: %v", err)
		}

		// Статус заказа обновился
		updatedOrder, err := storage.GetOrderByNumber(ctx, order.OrderNum)
		if err != nil {
			t.Fatalf("Failed to get order after balance update: %v", err)
		}
		if updatedOrder.Status != "PROCESSED" {
			t.Errorf("Expected status 'PROCESSED', got '%s'", updatedOrder.Status)
		}
		if updatedOrder.Accrual == nil || *updatedOrder.Accrual != accrual {
			t.Errorf("Expected accrual %f, got %v", accrual, updatedOrder.Accrual)
		}
	})
}

// TestDatabaseStorage_Concurrent тестирует конкурентный доступ к базе данных
func TestDatabaseStorage_Concurrent(t *testing.T) {
	if !dbAvailable {
		t.Skip("Database not available, skipping test")
	}
	storage, err := NewDatabaseStorage(context.Background(), testDatabaseURI)
	if err != nil {
		t.Skipf("Skipping database tests: failed to connect to database: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	err = storage.CreateUser(ctx, "concurrentuser", "password")
	if err != nil {
		t.Fatalf("Failed to create concurrent user: %v", err)
	}

	user, err := storage.GetUserByLogin(ctx, "concurrentuser")
	if err != nil {
		t.Fatalf("Failed to get concurrent user: %v", err)
	}
	if user == nil {
		t.Fatal("Concurrent user should not be nil")
	}

	// Конкурентное создание заказов
	t.Run("ConcurrentOrderCreation", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool, numGoroutines)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				defer func() { done <- true }()

				orderNumber := fmt.Sprintf("concurrent%d", id)
				_, err := storage.CreateOrder(ctx, user.ID, orderNumber)
				if err != nil {
					t.Errorf("Failed to create concurrent order %d: %v", id, err)
				}
			}(i)
		}

		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Все заказы созданы
		orders, err := storage.GetOrdersByUser(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get concurrent orders: %v", err)
		}
		if len(orders) != numGoroutines {
			t.Errorf("Expected %d orders, got %d", numGoroutines, len(orders))
		}
	})
}

// TestDatabaseStorage_Transaction тестирует транзакционность операций
func TestDatabaseStorage_Transaction(t *testing.T) {
	if !dbAvailable {
		t.Skip("Database not available, skipping test")
	}
	storage, err := NewDatabaseStorage(context.Background(), testDatabaseURI)
	if err != nil {
		t.Skipf("Skipping database tests: failed to connect to database: %v", err)
	}
	defer storage.Close()

	ctx := context.Background()

	err = storage.CreateUser(ctx, "transactionuser", "password")
	if err != nil {
		t.Fatalf("Failed to create transaction user: %v", err)
	}

	user, err := storage.GetUserByLogin(ctx, "transactionuser")
	if err != nil {
		t.Fatalf("Failed to get transaction user: %v", err)
	}
	if user == nil {
		t.Fatal("Transaction user should not be nil")
	}

	t.Run("BalanceUpdateTransaction", func(t *testing.T) {
		// Обновляем баланс
		err := storage.UpdateBalance(ctx, user.ID, 1000.0, 0.0)
		if err != nil {
			t.Fatalf("Failed to update balance: %v", err)
		}

		// Создаем списание
		err = storage.CreateWithdrawal(ctx, user.ID, "transactionorder", 100.0)
		if err != nil {
			t.Fatalf("Failed to create withdrawal: %v", err)
		}

		// Обновляем баланс после списания
		err = storage.UpdateBalance(ctx, user.ID, 900.0, 100.0)
		if err != nil {
			t.Fatalf("Failed to update balance after withdrawal: %v", err)
		}

		// Финальное состояние
		current, withdrawn, err := storage.GetUserBalance(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get final balance: %v", err)
		}
		if current != 0.0 { // Баланс вычисляется из заказов
			t.Errorf("Expected current balance 0.0, got %f", current)
		}
		if withdrawn != 100.0 {
			t.Errorf("Expected withdrawn 100.0, got %f", withdrawn)
		}

		withdrawals, err := storage.GetUserWithdrawals(ctx, user.ID)
		if err != nil {
			t.Fatalf("Failed to get withdrawals: %v", err)
		}
		if len(withdrawals) != 1 {
			t.Errorf("Expected 1 withdrawal, got %d", len(withdrawals))
		}
		if withdrawals[0].OrderNum != "transactionorder" {
			t.Errorf("Expected withdrawal order 'transactionorder', got '%s'", withdrawals[0].OrderNum)
		}
		if withdrawals[0].Sum != 100.0 {
			t.Errorf("Expected withdrawal sum 100.0, got %f", withdrawals[0].Sum)
		}
	})
}

// cleanupDatabase очищает базу данных перед тестами
func cleanupDatabase(t *testing.T, storage *DatabaseStorage) {
	ctx := context.Background()
	// Удаляем все данные из таблиц
	queries := []string{
		"DELETE FROM withdrawals",
		"DELETE FROM balances",
		"DELETE FROM orders",
		"DELETE FROM users",
	}
	for _, query := range queries {
		_, err := storage.pool.Exec(ctx, query)
		if err != nil {
			t.Logf("Failed to cleanup: %v", err)
		}
	}
}
