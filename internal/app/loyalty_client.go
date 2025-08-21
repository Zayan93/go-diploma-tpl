package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"go.uber.org/zap"
)

// LoyaltyService представляет клиент для работы с внешней системой расчёта баллов лояльности
type LoyaltyService struct {
	// Адрес внешнего сервиса лояльности
	AccrualSystemAddress string
	// HTTP-клиент с настройками
	Client *http.Client
}

// NewLoyaltyService создает новый экземпляр клиента для внешней системы лояльности
func NewLoyaltyService(accrualSystemAddress string) *LoyaltyService {
	return &LoyaltyService{
		AccrualSystemAddress: accrualSystemAddress,
		Client: &http.Client{
			Timeout: 30 * time.Second, // Таймаут для внешнего запроса
		},
	}
}

// OrderAccrualResponse представляет ответ от внешней системы лояльности
type OrderAccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// GetOrderAccrual делает HTTP-запрос к внешней системе лояльности
func (l *LoyaltyService) GetOrderAccrual(orderNum string) (*OrderAccrualResponse, error) {
	// Формируем URL для запроса к внешнему сервису
	url := fmt.Sprintf("%s/api/orders/%s", l.AccrualSystemAddress, orderNum)

	// Создаем HTTP-запрос
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Устанавливаем заголовки
	req.Header.Set("Content-Type", "application/json")

	// Выполняем запрос
	resp, err := l.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request to loyalty service: %w", err)
	}
	defer resp.Body.Close()

	// Читаем тело ответа
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Проверяем статус ответа
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("loyalty service returned status %d: %s", resp.StatusCode, string(body))
	}

	// Декодируем JSON-ответ
	var response OrderAccrualResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

// ProcessOrderAccrual обрабатывает заказ и обновляет начисление баллов
func (h *Handler) ProcessOrderAccrual(req *http.Request, orderNum string) error {
	// Получаем информацию о начислении от внешней системы лояльности
	loyaltyService := NewLoyaltyService(h.Config.AccrualSystemAddress)
	response, err := loyaltyService.GetOrderAccrual(orderNum)
	if err != nil {
		logger.Log.Error("Failed to get order accrual from external loyalty service", zap.Error(err))
		return err
	}

	// Если заказ обработан и есть начисление, обновляем его в базе
	if response.Status == "PROCESSED" && response.Accrual != nil {
		err = h.OrderStorage.UpdateOrderAccrual(req.Context(), orderNum, *response.Accrual)
		if err != nil {
			logger.Log.Error("Failed to update order accrual in database", zap.Error(err))
			return err
		}

		// Обновляем статус заказа
		order, err := h.OrderStorage.GetOrderByNumber(req.Context(), orderNum)
		if err != nil {
			logger.Log.Error("Failed to get order for status update", zap.Error(err))
			return err
		}

		if order != nil {
			err = h.OrderStorage.UpdateOrderStatus(req.Context(), order.ID, "PROCESSED")
			if err != nil {
				logger.Log.Error("Failed to update order status", zap.Error(err))
				return err
			}
		}

		logger.Log.Info("Order accrual processed successfully by external loyalty service",
			zap.String("orderNum", orderNum),
			zap.Float64("accrual", *response.Accrual))
	} else {
		logger.Log.Info("Order not eligible for accrual according to external loyalty service",
			zap.String("orderNum", orderNum),
			zap.String("status", response.Status))
	}

	return nil
}

// MockLoyaltyHandler обрабатывает запросы к внешней системе лояльности
// Этот хендлер делает запросы к внешнему сервису лояльности
func (h *Handler) MockLoyaltyHandler(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Извлекаем номер заказа из URL
	orderNum := req.URL.Path[len("/api/orders/"):]
	if orderNum == "" {
		http.Error(res, "Order number is required", http.StatusBadRequest)
		return
	}

	// Получаем информацию о начислении от внешней системы лояльности
	loyaltyService := NewLoyaltyService(h.Config.AccrualSystemAddress)
	response, err := loyaltyService.GetOrderAccrual(orderNum)
	if err != nil {
		logger.Log.Error("Failed to get order accrual from external loyalty service", zap.Error(err))
		http.Error(res, "Failed to get information from loyalty service", http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовки
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	// Кодируем ответ в JSON
	if err := json.NewEncoder(res).Encode(response); err != nil {
		logger.Log.Error("Failed to encode loyalty response", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("External loyalty service response forwarded", zap.String("orderNum", orderNum))
}
