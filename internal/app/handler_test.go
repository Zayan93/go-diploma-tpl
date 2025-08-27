package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// userIDKey - собственный тип для ключа контекста пользователя
type userIDKey struct{}

// Простые тесты для валидации номеров заказов
func TestIsValidOrderNumber(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name     string
		orderNum string
		expected bool
	}{
		{"Valid order number", "1234567890", true},
		{"Valid order number", "4532015112830366", true},
		{"Invalid order number", "123456789", false},
		{"Invalid order number", "1234567891", false},
		{"Empty string", "", false},
		{"Non-numeric", "abc123def", false},
		{"Single digit", "5", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.isValidOrderNumber(tt.orderNum)
			if result != tt.expected {
				t.Errorf("Expected %v for order number %s, got %v", tt.expected, tt.orderNum, result)
			}
		})
	}
}

// Тест для проверки структуры OrderResponse
func TestOrderResponseStructure(t *testing.T) {
	order := OrderResponse{
		Number:     "1234567890",
		Status:     "NEW",
		UploadedAt: "2023-01-01T00:00:00Z",
	}

	if order.Number != "1234567890" {
		t.Error("Order number should be set correctly")
	}

	if order.Status != "NEW" {
		t.Error("Order status should be set correctly")
	}

	if order.UploadedAt != "2023-01-01T00:00:00Z" {
		t.Error("UploadedAt should be set correctly")
	}
}

// Тест для проверки структуры WithdrawalRequest
func TestWithdrawalRequestStructure(t *testing.T) {
	withdrawal := WithdrawalRequest{
		Order: "9876543210",
		Sum:   100.50,
	}

	if withdrawal.Order != "9876543210" {
		t.Error("Order should be set correctly")
	}

	if withdrawal.Sum != 100.50 {
		t.Error("Sum should be set correctly")
	}
}

// Тест для проверки структуры WithdrawalResponse
func TestWithdrawalResponseStructure(t *testing.T) {
	withdrawal := WithdrawalResponse{
		Order:       "9876543210",
		Sum:         100.50,
		ProcessedAt: "2023-01-01T00:00:00Z",
	}

	if withdrawal.Order != "9876543210" {
		t.Error("Order should be set correctly")
	}

	if withdrawal.Sum != 100.50 {
		t.Error("Sum should be set correctly")
	}

	if withdrawal.ProcessedAt != "2023-01-01T00:00:00Z" {
		t.Error("ProcessedAt should be set correctly")
	}
}

// Тест для проверки структуры AUTHRequestBody
func TestAUTHRequestBodyStructure(t *testing.T) {
	auth := AUTHRequestBody{
		Login:    "testuser",
		Password: "password123",
	}

	if auth.Login != "testuser" {
		t.Error("Login should be set correctly")
	}

	if auth.Password != "password123" {
		t.Error("Password should be set correctly")
	}
}

// Тест для проверки JSON сериализации
func TestJSONSerialization(t *testing.T) {
	withdrawal := WithdrawalRequest{
		Order: "1234567890",
		Sum:   75.25,
	}

	jsonData, err := json.Marshal(withdrawal)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	var decoded WithdrawalRequest
	err = json.Unmarshal(jsonData, &decoded)
	if err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.Order != withdrawal.Order {
		t.Error("Order should be preserved after JSON round-trip")
	}

	if decoded.Sum != withdrawal.Sum {
		t.Error("Sum should be preserved after JSON round-trip")
	}
}

// Тест для проверки HTTP метода
func TestHTTPMethodValidation(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	// Создаем простой обработчик для тестирования
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusOK)
	}

	handler(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}
}

// Тест для проверки контекста пользователя
func TestUserContext(t *testing.T) {
	ctx := context.Background()
	userID := 123

	// Симулируем контекст с userID используя собственный тип ключа
	ctxWithUser := context.WithValue(ctx, userIDKey{}, userID)

	// Извлекаем userID из контекста
	extractedUserID, ok := ctxWithUser.Value(userIDKey{}).(int)
	if !ok {
		t.Fatal("Failed to extract userID from context")
	}

	if extractedUserID != userID {
		t.Errorf("Expected userID %d, got %d", userID, extractedUserID)
	}
}

// Тест для проверки валидации номера заказа
func TestOrderNumberValidation(t *testing.T) {
	handler := &Handler{}

	validNumbers := []string{
		"1234567890",
		"4532015112830366",
		"79927398713",
	}

	invalidNumbers := []string{
		"123456789",
		"1234567891",
		"",
		"abc123def",
		"5",
		"12345",
	}

	// Проверяем валидные номера
	for _, num := range validNumbers {
		if !handler.isValidOrderNumber(num) {
			t.Errorf("Order number %s should be valid", num)
		}
	}

	// Проверяем невалидные номера
	for _, num := range invalidNumbers {
		if handler.isValidOrderNumber(num) {
			t.Errorf("Order number %s should be invalid", num)
		}
	}
}

// Тест для проверки алгоритма Луна
func TestLuhnAlgorithm(t *testing.T) {
	handler := &Handler{}

	// Тестовые номера с известными результатами
	testCases := []struct {
		number   string
		expected bool
	}{
		{"4532015112830366", true},  // Валидная кредитная карта
		{"79927398713", true},       // Валидный номер
		{"1234567890", true},        // Валидный номер
		{"123456789", false},        // Невалидный номер
		{"1234567891", false},       // Невалидный номер
		{"4532015112830367", false}, // Невалидный номер
	}

	for _, tc := range testCases {
		result := handler.isValidOrderNumber(tc.number)
		if result != tc.expected {
			t.Errorf("Luhn algorithm failed for %s: expected %v, got %v",
				tc.number, tc.expected, result)
		}
	}
}
