package app

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/Zayan93/go-diploma-tpl/internal/config"
	"github.com/Zayan93/go-diploma-tpl/internal/logger"
	"github.com/Zayan93/go-diploma-tpl/internal/middleware"
	"github.com/Zayan93/go-diploma-tpl/internal/services"
	"github.com/Zayan93/go-diploma-tpl/internal/storage"
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

func NewHandler(userStorage *storage.DatabaseStorage, authService *services.AuthService, accrualService *services.AccrualService, cfg *config.Config) *Handler {
	return &Handler{
		UserStorage:    userStorage,
		OrderStorage:   userStorage, // DatabaseStorage реализует оба интерфейса
		AuthService:    authService,
		AccrualService: accrualService,
		Config:         cfg,
	}
}

type Handler struct {
	UserStorage    *storage.DatabaseStorage
	OrderStorage   *storage.DatabaseStorage
	AuthService    *services.AuthService
	AccrualService *services.AccrualService
	Config         *config.Config // добавляем конфигурацию
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
	exists, err := h.UserStorage.UserExists(req.Context(), requestBody.Login)
	if err != nil {
		logger.Log.Error("Failed to check if user exists", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(res, "User with this login already exists", http.StatusConflict)
		return
	}

	// Хешируем пароль с помощью bcrypt
	hashedPassword, err := h.AuthService.HashPassword(requestBody.Password)
	if err != nil {
		logger.Log.Error("Failed to hash password", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Создаем пользователя
	if err := h.UserStorage.CreateUser(req.Context(), requestBody.Login, hashedPassword); err != nil {
		logger.Log.Error("Failed to create user", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Получаем созданного пользователя для генерации токена
	user, err := h.UserStorage.GetUserByLogin(req.Context(), requestBody.Login)
	if err != nil {
		logger.Log.Error("Failed to get created user", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Генерируем JWT токен
	token, err := h.AuthService.GenerateJWT(user.ID, user.Login)
	if err != nil {
		logger.Log.Error("Failed to generate JWT token", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Устанавливаем токен в заголовок
	res.Header().Set("Authorization", "Bearer "+token)
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
	user, err := h.UserStorage.GetUserByLogin(req.Context(), requestBody.Login)
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

	// Проверяем пароль с помощью bcrypt
	if err := h.AuthService.CheckPassword(user.Password, requestBody.Password); err != nil {
		http.Error(res, "Invalid login or password", http.StatusUnauthorized)
		return
	}

	// Генерируем JWT токен
	token, err := h.AuthService.GenerateJWT(user.ID, user.Login)
	if err != nil {
		logger.Log.Error("Failed to generate JWT token", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Устанавливаем токен в заголовок
	res.Header().Set("Authorization", "Bearer "+token)
	logger.Log.Info("User logged in successfully", zap.String("login", requestBody.Login))
	res.WriteHeader(http.StatusOK)
}

// PostOrders обрабатывает загрузку номера заказа пользователем
func (h *Handler) PostOrders(res http.ResponseWriter, req *http.Request) {
	// Проверяем аутентификацию пользователя
	userID, ok := middleware.GetUserIDFromContext(req.Context())
	if !ok {
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	logger.Log.Info("User authenticated for order upload", zap.Int("userID", userID))

	// Читаем номер заказа из тела запроса
	body, err := io.ReadAll(req.Body)
	if err != nil {
		logger.Log.Error("Failed to read request body", zap.Error(err))
		http.Error(res, "Invalid request format", http.StatusBadRequest)
		return
	}
	defer req.Body.Close()

	orderNum := string(body)
	if orderNum == "" {
		http.Error(res, "Order number is required", http.StatusBadRequest)
		return
	}

	logger.Log.Info("Processing order number", zap.String("orderNum", orderNum))

	// Проверяем формат номера заказа (должен быть числом)
	if !h.isValidOrderNumber(orderNum) {
		http.Error(res, "Invalid order number format", http.StatusUnprocessableEntity)
		return
	}

	// Проверяем, существует ли уже заказ с таким номером
	existingOrder, err := h.OrderStorage.GetOrderByNumber(req.Context(), orderNum)
	if err != nil {
		logger.Log.Error("Failed to check if order exists", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Checking existing order", zap.String("orderNum", orderNum), zap.Any("existingOrder", existingOrder))

	if existingOrder != nil {
		// Заказ уже существует
		if existingOrder.UserID == userID {
			// Заказ уже был загружен этим пользователем
			logger.Log.Info("Order already uploaded by this user", zap.String("orderNum", orderNum), zap.Int("userID", userID))
			res.WriteHeader(http.StatusOK)
		} else {
			// Заказ уже был загружен другим пользователем
			logger.Log.Info("Order already uploaded by another user", zap.String("orderNum", orderNum), zap.Int("userID", userID))
			http.Error(res, "Order already uploaded by another user", http.StatusConflict)
		}
		return
	}

	// Создаем новый заказ со статусом NEW (без начисления баллов)
	_, err = h.OrderStorage.CreateOrder(req.Context(), userID, orderNum)
	if err != nil {
		logger.Log.Error("Failed to create order", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Order created successfully", zap.String("orderNum", orderNum), zap.Int("userID", userID))
	res.WriteHeader(http.StatusAccepted)
	res.Write([]byte{}) // Пустой ответ для завершения
}

// GetOrders возвращает список заказов пользователя
func (h *Handler) GetOrders(res http.ResponseWriter, req *http.Request) {
	// Проверяем аутентификацию пользователя
	userID, ok := middleware.GetUserIDFromContext(req.Context())
	if !ok {
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем заказы пользователя
	orders, err := h.OrderStorage.GetOrdersByUser(req.Context(), userID)
	if err != nil {
		logger.Log.Error("Failed to get user orders", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Устанавливаем заголовок Content-Type
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)

	// Если нет заказов, возвращаем пустой массив
	if len(orders) == 0 {
		res.Write([]byte("[]"))
		logger.Log.Info("Orders retrieved successfully", zap.Int("userID", userID), zap.Int("count", 0))
		return
	}

	// Преобразуем заказы в формат ответа API
	var orderResponses []OrderResponse
	for _, order := range orders {
		orderResponse := OrderResponse{
			Number:     order.OrderNum,
			Status:     order.Status,
			Accrual:    order.Accrual,
			UploadedAt: order.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}
		orderResponses = append(orderResponses, orderResponse)
	}

	// Кодируем ответ в JSON
	if err := json.NewEncoder(res).Encode(orderResponses); err != nil {
		logger.Log.Error("Failed to encode orders response", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	logger.Log.Info("Orders retrieved successfully", zap.Int("userID", userID), zap.Int("count", len(orderResponses)))
}

// GetUserBalance возвращает текущий баланс пользователя
func (h *Handler) GetUserBalance(res http.ResponseWriter, req *http.Request) {
	if req.Method != http.MethodGet {
		http.Error(res, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Проверяем аутентификацию пользователя
	userID, ok := middleware.GetUserIDFromContext(req.Context())
	if !ok {
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем баланс пользователя
	current, withdrawn, err := h.OrderStorage.GetUserBalance(req.Context(), userID)
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
		Current:   current - withdrawn, // Доступный баланс = начисленный - снятый
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
	userID, ok := middleware.GetUserIDFromContext(req.Context())
	if !ok {
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

	// Получаем текущий баланс пользователя
	current, withdrawn, err := h.OrderStorage.GetUserBalance(req.Context(), userID)
	if err != nil {
		logger.Log.Error("Failed to get user balance", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Доступный баланс = начисленный - снятый
	availableBalance := current - withdrawn

	// Проверяем, достаточно ли средств
	if availableBalance < requestBody.Sum {
		http.Error(res, "Insufficient funds", http.StatusPaymentRequired)
		return
	}

	// Создаем вывод средств
	err = h.OrderStorage.CreateWithdrawal(req.Context(), userID, requestBody.Order, requestBody.Sum)
	if err != nil {
		logger.Log.Error("Failed to create withdrawal", zap.Error(err))
		http.Error(res, "Internal server error", http.StatusInternalServerError)
		return
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
	userID, ok := middleware.GetUserIDFromContext(req.Context())
	if !ok {
		http.Error(res, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Получаем выводы средств пользователя
	withdrawals, err := h.OrderStorage.GetUserWithdrawals(req.Context(), userID)
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
			ProcessedAt: withdrawal.ProcessedAt.Format("2006-01-02T15:04:05Z"),
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

// isValidOrderNumber проверяет, является ли номер заказа валидным
func (h *Handler) isValidOrderNumber(orderNum string) bool {
	// Проверяем, что строка состоит только из цифр
	if !regexp.MustCompile(`^\d+$`).MatchString(orderNum) {
		return false
	}

	// Алгоритм Луна
	sum := 0
	alternate := false

	for i := len(orderNum) - 1; i >= 0; i-- {
		digit, err := strconv.Atoi(string(orderNum[i]))
		if err != nil {
			return false
		}

		if alternate {
			digit *= 2
			if digit > 9 {
				digit = (digit % 10) + 1
			}
		}

		sum += digit
		alternate = !alternate
	}

	return sum%10 == 0
}
