package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

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

// OrderResponse представляет заказ в ответе API
type OrderResponse struct {
	Number     string   `json:"number"`
	Status     string   `json:"status"`
	Accrual    *float64 `json:"accrual,omitempty"`
	UploadedAt string   `json:"uploaded_at"`
}

func NewHandler(userStorage store.UserStorage) *Handler {
	return &Handler{
		UserStorage:  userStorage,
		OrderStorage: userStorage.(store.OrderStorage), // Приводим к OrderStorage
	}
}

type Handler struct {
	UserStorage  store.UserStorage
	OrderStorage store.OrderStorage
}

// hashPassword хеширует пароль с использованием SHA-256
func hashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return fmt.Sprintf("%x", hash)
}

// generateSessionID генерирует уникальный идентификатор сессии
func generateSessionID() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// setAuthCookie устанавливает куки для аутентификации
func (h *Handler) setAuthCookie(res http.ResponseWriter, userID string) {
	sessionID := generateSessionID()
	cookie := &http.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour), // Сессия на 24 часа
	}
	http.SetCookie(res, cookie)
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

	// Устанавливаем куки для автоматической аутентификации
	h.setAuthCookie(res, fmt.Sprintf("%d", user.ID))

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

	// Устанавливаем куки для аутентификации
	h.setAuthCookie(res, fmt.Sprintf("%d", user.ID))

	logger.Log.Info("User logged in successfully", zap.String("login", requestBody.Login))
	res.WriteHeader(http.StatusOK)
}

// PostShorten - хендлер для сокращения URL (заглушка для совместимости)
func (h *Handler) PostShorten(res http.ResponseWriter, req *http.Request) {
	// Этот хендлер оставлен для совместимости, но требует доработки
	// для работы с URL сокращением
	res.WriteHeader(http.StatusNotImplemented)
	res.Write([]byte("URL shortening functionality not implemented yet"))
}

// PostOrders обрабатывает загрузку номера заказа пользователем
func (h *Handler) PostOrders(res http.ResponseWriter, req *http.Request) {
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

	// Читаем номер заказа из тела запроса
	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Error("Failed to read request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	orderNum := strings.TrimSpace(string(body))
	if orderNum == "" {
		http.Error(res, "Order number is required", http.StatusBadRequest)
		return
	}

	// Проверяем формат номера заказа (должен быть числом)
	if !h.isValidOrderNumber(orderNum) {
		http.Error(res, "Invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	// Проверяем, существует ли уже заказ с таким номером
	existingOrder, err := h.OrderStorage.GetOrderByNumber(orderNum)
	if err != nil {
		logger.Log.Error("Failed to check if order exists", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if existingOrder != nil {
		// Заказ уже существует
		if existingOrder.UserID == userID {
			// Заказ уже был загружен этим пользователем
			logger.Log.Info("Order already uploaded by this user", zap.String("orderNum", orderNum), zap.Int("userID", userID))
			res.WriteHeader(http.StatusOK)
			return
		} else {
			// Заказ уже был загружен другим пользователем
			logger.Log.Info("Order already uploaded by another user", zap.String("orderNum", orderNum), zap.Int("userID", userID))
			http.Error(res, "Order already uploaded by another user", http.StatusConflict)
			return
		}
	}

	// Создаем новый заказ
	err = h.OrderStorage.CreateOrder(userID, orderNum)
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

	// Получаем заказы пользователя
	orders, err := h.OrderStorage.GetOrdersByUser(userID)
	if err != nil {
		logger.Log.Error("Failed to get user orders", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Преобразуем заказы в формат ответа API
	var orderResponses []OrderResponse
	for _, order := range orders {
		orderResponse := OrderResponse{
			Number:     order.OrderNum,
			Status:     order.Status,
			Accrual:    order.Accrual,
			UploadedAt: order.CreatedAt,
		}
		orderResponses = append(orderResponses, orderResponse)
	}

	// Устанавливаем заголовок Content-Type
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	// Кодируем ответ в JSON
	if err := json.NewEncoder(res).Encode(orderResponses); err != nil {
		logger.Log.Error("Failed to encode orders response", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Orders retrieved successfully", zap.Int("userID", userID), zap.Int("count", len(orderResponses)))
}

// getUserIDFromCookie извлекает ID пользователя из куки
func (h *Handler) getUserIDFromCookie(req *http.Request) (int, error) {
	_, err := req.Cookie("session_id")
	if err != nil {
		return 0, err
	}

	// TODO: Реализовать проверку сессии и получение userID
	// Пока что возвращаем заглушку
	// В реальной реализации здесь должна быть проверка сессии
	return 1, nil
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
