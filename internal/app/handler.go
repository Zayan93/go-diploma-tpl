package app

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/Zayan93/go-diploma-tpl/internal/config"
	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"github.com/Zayan93/go-diploma-tpl/internal/store"
	"go.uber.org/zap"
)

type AUTHRequestBody struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AUTHResponseBody struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// WithdrawalRequest представляет запрос на вывод средств
type WithdrawalRequest struct {
	Order string  `json:"order"`
	Sum   float64 `json:"sum"`
}

// WithdrawalResponse представляет ответ с информацией о выводе средств
type WithdrawalResponse struct {
	Order       string  `json:"order"`
	Sum         float64 `json:"sum"`
	ProcessedAt string  `json:"processed_at"`
}

// OrderResponse представляет заказ в ответе API
type OrderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

func NewHandler(userStorage store.UserStorage, cfg *config.Config) *Handler {
	return &Handler{
		UserStorage:  userStorage,
		OrderStorage: userStorage.(store.OrderStorage), // Приводим к OrderStorage
		Config:       cfg,
	}
}

type Handler struct {
	UserStorage  store.UserStorage
	OrderStorage store.OrderStorage
	Config       *config.Config // добавляем конфигурацию
}

// hashPassword хеширует пароль с использованием SHA-256
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}

// PostRegister обрабатывает регистрацию пользователя
func (h *Handler) PostRegister(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody AUTHRequestBody
	if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
		logger.Log.Error("Failed to decode request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что логин и пароль не пустые
	if requestBody.Login == "" || requestBody.Password == "" {
		http.Error(res, "Login and password are required", http.StatusBadRequest)
		return
	}

	// Проверяем, существует ли уже пользователь с таким логином
	exists, err := h.UserStorage.UserExists(requestBody.Login)
	if err != nil {
		logger.Log.Error("Failed to check if user exists", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(res, "User with this login already exists", http.StatusConflict)
		return
	}

	// Хешируем пароль
	hashedPassword := hashPassword(requestBody.Password)

	// Создаем пользователя
	if err := h.UserStorage.CreateUser(requestBody.Login, hashedPassword); err != nil {
		logger.Log.Error("Failed to create user", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Получаем созданного пользователя для установки куки
	user, err := h.UserStorage.GetUserByLogin(requestBody.Login)
	if err != nil {
		logger.Log.Error("Failed to get created user", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	cookie := &http.Cookie{
		Name:     "user_id",
		Value:    fmt.Sprintf("%d", user.ID),
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(res, cookie)

	logger.Log.Info("User registered successfully", zap.String("login", requestBody.Login))
	res.WriteHeader(http.StatusOK)
}

// PostLogin обрабатывает аутентификацию пользователя
func (h *Handler) PostLogin(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var requestBody AUTHRequestBody
	if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
		logger.Log.Error("Failed to decode request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что логин и пароль не пустые
	if requestBody.Login == "" || requestBody.Password == "" {
		http.Error(res, "Login and password are required", http.StatusBadRequest)
		return
	}

	// Получаем пользователя по логину
	user, err := h.UserStorage.GetUserByLogin(requestBody.Login)
	if err != nil {
		logger.Log.Error("Failed to get user by login", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Проверяем, существует ли пользователь
	if user == nil {
		http.Error(res, "Invalid login or password", http.StatusUnauthorized)
		return
	}

	// Проверяем пароль
	hashedPassword := hashPassword(requestBody.Password)
	if user.Password != hashedPassword {
		http.Error(res, "Invalid login or password", http.StatusUnauthorized)
		return
	}

	cookie := &http.Cookie{
		Name:     "user_id",
		Value:    fmt.Sprintf("%d", user.ID),
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}
	http.SetCookie(res, cookie)

	logger.Log.Info("User logged in successfully", zap.String("login", requestBody.Login))
	res.WriteHeader(http.StatusOK)
}

// PostOrders обрабатывает загрузку номера заказа пользователем
func (h *Handler) PostOrders(res http.ResponseWriter, req *http.Request) {
	// Проверяем аутентификацию пользователя
	userID, err := h.getUserIDFromCookie(req)
	if err != nil {
		logger.Log.Error("Failed to get user ID from cookie", zap.Error(err))
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Читаем номер заказа из тела запроса
	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Error("Failed to read request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	orderNum := string(body)

	// Проверяем формат номера заказа (должен быть числом)
	if !h.isValidOrderNumber(orderNum) {
		http.Error(res, "Invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	// Проверяем, существует ли уже заказ с таким номером
	existingOrder, err := h.OrderStorage.GetOrderByNumber(orderNum)
	if err != nil {
		logger.Log.Error("Failed to check if order exists", zap.Error(err))
		http.Error(res, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	if existingOrder != nil {
		// Заказ уже существует
		if existingOrder.UserID == userID {
			// Заказ уже был загружен этим пользователем
			res.WriteHeader(http.StatusOK)
		} else {
			http.Error(res, "Order already uploaded by another user", http.StatusConflict)
		}
		return
	}

	// Создаем новый заказ (начисление баллов происходит автоматически в CreateOrder)
	_, err = h.OrderStorage.CreateOrder(userID, orderNum)
	if err != nil {
		logger.Log.Error("Failed to create order", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Order created successfully", zap.String("orderNum", orderNum), zap.Int("userID", userID))
	res.WriteHeader(http.StatusAccepted)
}

// GetOrders возвращает список заказов пользователя
func (h *Handler) GetOrders(res http.ResponseWriter, req *http.Request) {
	// Проверяем аутентификацию пользователя
	userID, err := h.getUserIDFromCookie(req)
	if err != nil {
		logger.Log.Error("Failed to get user ID from cookie", zap.Error(err))
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем заказы пользователя
	orders, err := h.OrderStorage.GetOrdersByUser(userID)
	if err != nil {
		logger.Log.Error("Failed to get user orders", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		res.Header().Set("Content-Type", "application/json")
		res.Write([]byte("[]"))
		return
	}

	// Устанавливаем заголовок Content-Type
	res.Header().Set("Content-Type", "application/json")
	json.NewEncoder(res).Encode(orders)
}

// GetUserBalance возвращает текущий баланс пользователя
func (h *Handler) GetUserBalance(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем аутентификацию пользователя
	userID, err := h.getUserIDFromCookie(req)
	if err != nil {
		logger.Log.Error("Failed to get user ID from cookie", zap.Error(err))
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем баланс пользователя
	current, withdrawn, err := h.OrderStorage.GetUserBalance(userID)
	if err != nil {
		logger.Log.Error("Failed to get user balance", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Создаем ответ
	balanceResponse := struct {
		Current   float64 `json:"current"`
		Withdrawn float64 `json:"withdrawn"`
	}{
		Current:   current,
		Withdrawn: withdrawn,
	}

	// Устанавливаем заголовок Content-Type
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	// Кодируем ответ в JSON
	if err := json.NewEncoder(res).Encode(balanceResponse); err != nil {
		logger.Log.Error("Failed to encode balance response", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("User balance retrieved successfully", zap.Int("userID", userID), zap.Float64("current", current), zap.Float64("withdrawn", withdrawn))
}

// PostWithdrawBalance обрабатывает запрос на вывод средств
func (h *Handler) PostWithdrawBalance(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodPost {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем аутентификацию пользователя
	userID, err := h.getUserIDFromCookie(req)
	if err != nil {
		logger.Log.Error("Failed to get user ID from cookie", zap.Error(err))
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Декодируем тело запроса
	var requestBody WithdrawalRequest
	if err := json.NewDecoder(req.Body).Decode(&requestBody); err != nil {
		logger.Log.Error("Failed to decode request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	// Проверяем, что номер заказа и сумма не пустые
	if requestBody.Order == "" || requestBody.Sum <= 0 {
		http.Error(res, "Order number and positive sum are required", http.StatusBadRequest)
		return
	}

	// Проверяем формат номера заказа (должен быть числом)
	if !h.isValidOrderNumber(requestBody.Order) {
		http.Error(res, "Invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	// Проверяем, существует ли уже заказ с таким номером
	existingOrder, err := h.OrderStorage.GetOrderByNumber(requestBody.Order)
	if err != nil {
		logger.Log.Error("Failed to check if order exists", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if existingOrder != nil {
		// Заказ уже существует
		if existingOrder.UserID == userID {
			// Заказ уже был загружен этим пользователем
			http.Error(res, "Order already uploaded by this user", http.StatusConflict)
			return
		} else {
			// Заказ уже был загружен другим пользователем
			http.Error(res, "Order already uploaded by another user", http.StatusConflict)
			return
		}
	}

	// Получаем текущий баланс пользователя
	current, _, err := h.OrderStorage.GetUserBalance(userID)
	if err != nil {
		logger.Log.Error("Failed to get user balance", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Проверяем, достаточно ли средств
	if current < requestBody.Sum {
		http.Error(res, "Insufficient funds", http.StatusPaymentRequired)
		return
	}

	// Создаем вывод средств
	err = h.OrderStorage.CreateWithdrawal(userID, requestBody.Order, requestBody.Sum)
	if err != nil {
		logger.Log.Error("Failed to create withdrawal", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Создаем заказ с отрицательным начислением (списание)
	_, err = h.OrderStorage.CreateOrder(userID, requestBody.Order)
	if err != nil {
		logger.Log.Error("Failed to create order for withdrawal", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Устанавливаем отрицательное начисление (списание)
	err = h.OrderStorage.UpdateOrderAccrual(requestBody.Order, -requestBody.Sum)
	if err != nil {
		logger.Log.Error("Failed to update order accrual for withdrawal", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Обновляем статус заказа
	order, err := h.OrderStorage.GetOrderByNumber(requestBody.Order)
	if err != nil {
		logger.Log.Error("Failed to get order for status update", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if order != nil {
		err = h.OrderStorage.UpdateOrderStatus(order.ID, "PROCESSED")
		if err != nil {
			logger.Log.Error("Failed to update order status", zap.Error(err))
			http.Error(res, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	logger.Log.Info("Withdrawal created successfully",
		zap.Int("userID", userID),
		zap.String("orderNum", requestBody.Order),
		zap.Float64("sum", requestBody.Sum))
	res.WriteHeader(http.StatusOK)
}

// GetUserWithdrawals возвращает информацию о выводах средств пользователя
func (h *Handler) GetUserWithdrawals(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем аутентификацию пользователя
	userID, err := h.getUserIDFromCookie(req)
	if err != nil {
		logger.Log.Error("Failed to get user ID from cookie", zap.Error(err))
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем выводы средств пользователя
	withdrawals, err := h.OrderStorage.GetUserWithdrawals(userID)
	if err != nil {
		logger.Log.Error("Failed to get user withdrawals", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Если нет выводов, возвращаем 204
	if len(withdrawals) == 0 {
		res.WriteHeader(http.StatusNoContent)
		return
	}

	// Преобразуем выводы в формат ответа API
	var withdrawalResponses []WithdrawalResponse
	for _, withdrawal := range withdrawals {
		withdrawalResponse := WithdrawalResponse{
			Order:       withdrawal.OrderNum,
			Sum:         withdrawal.Sum,
			ProcessedAt: withdrawal.ProcessedAt,
		}
		withdrawalResponses = append(withdrawalResponses, withdrawalResponse)
	}

	// Устанавливаем заголовок Content-Type
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	// Кодируем ответ в JSON
	if err := json.NewEncoder(res).Encode(withdrawalResponses); err != nil {
		logger.Log.Error("Failed to encode withdrawals response", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("User withdrawals retrieved successfully", zap.Int("userID", userID), zap.Int("count", len(withdrawalResponses)))
}

// getUserIDFromCookie извлекает ID пользователя из куки
func (h *Handler) getUserIDFromCookie(req *http.Request) (int, error) {
	cookie, err := req.Cookie("user_id")
	if err != nil {
		// Куки нет, значит пользователь не авторизован
		return 0, err
	}

	// Кука есть, возвращаем существующий ID
	userID, err := strconv.Atoi(cookie.Value)
	if err != nil {
		logger.Log.Error("Failed to parse user ID from cookie", zap.String("cookieValue", cookie.Value), zap.Error(err))
		return 0, fmt.Errorf("invalid user ID in cookie")
	}

	logger.Log.Info("Using existing user ID", zap.Int("userID", userID))
	return userID, nil
}

// isValidOrderNumber проверяет, является ли номер заказа валидным
func (h *Handler) isValidOrderNumber(orderNum string) bool {
	// Проверяем, что строка состоит только из цифр
	for _, char := range orderNum {
		if char < '0' || char > '9' {
			return false
		}
	}
	return len(orderNum) > 0
}
