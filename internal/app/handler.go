package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
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

func NewHandler(userStorage store.UserStorage) *Handler {
	return &Handler{
		UserStorage: userStorage,
	}
}

type Handler struct {
	UserStorage store.UserStorage
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
