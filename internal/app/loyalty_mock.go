package app

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"time"

	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"go.uber.org/zap"
)

// LoyaltyMockService представляет заглушку для системы расчёта баллов лояльности
type LoyaltyMockService struct {
	// В реальной системе здесь были бы настройки для подключения к внешнему сервису
	AccrualSystemAddress string // адрес системы расчёта начислений
}

// NewLoyaltyMockService создает новый экземпляр заглушки
func NewLoyaltyMockService(accrualSystemAddress string) *LoyaltyMockService {
	return &LoyaltyMockService{
		AccrualSystemAddress: accrualSystemAddress,
	}
}

// OrderAccrualResponse представляет ответ от системы лояльности
type OrderAccrualResponse struct {
	Order   string   `json:"order"`
	Status  string   `json:"status"`
	Accrual *float64 `json:"accrual,omitempty"`
}

// GetOrderAccrual возвращает информацию о начислении баллов для заказа
// Это заглушка, которая имитирует работу внешней системы лояльности
func (l *LoyaltyMockService) GetOrderAccrual(orderNum string) (*OrderAccrualResponse, error) {
	// Имитируем задержку внешнего сервиса
	time.Sleep(100 * time.Millisecond)

	// Генерируем случайный статус и начисление
	rand.Seed(time.Now().UnixNano())

	// 70% заказов получают начисление, 30% - нет
	if rand.Float64() < 0.7 {
		// Генерируем случайное начисление от 1 до 100 баллов
		accrual := rand.Float64()*99 + 1
		// Округляем до 2 знаков после запятой
		accrual = float64(int(accrual*100)) / 100

		return &OrderAccrualResponse{
			Order:   orderNum,
			Status:  "PROCESSED",
			Accrual: &accrual,
		}, nil
	} else {
		return &OrderAccrualResponse{
			Order:  orderNum,
			Status: "INVALID",
		}, nil
	}
}

// ProcessOrderAccrual обрабатывает заказ и обновляет начисление баллов
func (h *Handler) ProcessOrderAccrual(orderNum string) error {
	// Получаем информацию о начислении от системы лояльности
	loyaltyService := NewLoyaltyMockService(h.Config.AccrualSystemAddress)
	response, err := loyaltyService.GetOrderAccrual(orderNum)
	if err != nil {
		logger.Log.Error("Failed to get order accrual from loyalty service", zap.Error(err))
		return err
	}

	// Если заказ обработан и есть начисление, обновляем его в базе
	if response.Status == "PROCESSED" && response.Accrual != nil {
		err = h.OrderStorage.UpdateOrderAccrual(orderNum, *response.Accrual)
		if err != nil {
			logger.Log.Error("Failed to update order accrual in database", zap.Error(err))
			return err
		}

		// Обновляем статус заказа
		order, err := h.OrderStorage.GetOrderByNumber(orderNum)
		if err != nil {
			logger.Log.Error("Failed to get order for status update", zap.Error(err))
			return err
		}

		if order != nil {
			err = h.OrderStorage.UpdateOrderStatus(order.ID, "PROCESSED")
			if err != nil {
				logger.Log.Error("Failed to update order status", zap.Error(err))
				return err
			}
		}

		logger.Log.Info("Order accrual processed successfully",
			zap.String("orderNum", orderNum),
			zap.Float64("accrual", *response.Accrual))
	} else {
		logger.Log.Info("Order not eligible for accrual",
			zap.String("orderNum", orderNum),
			zap.String("status", response.Status))
	}

	return nil
}

// MockLoyaltyHandler обрабатывает запросы к заглушке системы лояльности
// Этот хендлер имитирует внешний API системы лояльности
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

	// Получаем информацию о начислении
	loyaltyService := NewLoyaltyMockService(h.Config.AccrualSystemAddress)
	response, err := loyaltyService.GetOrderAccrual(orderNum)
	if err != nil {
		logger.Log.Error("Failed to get order accrual", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
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

	logger.Log.Info("Loyalty mock response sent", zap.String("orderNum", orderNum))
}
