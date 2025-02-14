package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"clinicms/models"

	"gorm.io/gorm"
)

var Broadcast = make(chan interface{})

func CreateChat(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, err := strconv.Atoi(r.Header.Get("UserID"))
		if err != nil {
			http.Error(w, `{"error": "Invalid user ID"}`, http.StatusUnauthorized)
			return
		}

		role := r.Header.Get("Role")
		if role != "admin" {
			role = "client"
		}

		var existingChat models.Chat
		if err := db.Where("client_id = ? AND status = 'active'", userID).First(&existingChat).Error; err == nil {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":        existingChat.ID,
				"client_id": existingChat.ClientID,
				"status":    existingChat.Status,
			})
			return
		}

		newChat := models.Chat{
			ClientID:  uint(userID),
			Status:    "active",
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}

		if err := db.Create(&newChat).Error; err != nil {
			http.Error(w, `{"error": "Failed to create chat"}`, http.StatusInternalServerError)
			return
		}

		type WebSocketMessage struct {
			Type string      `json:"type"`
			Chat models.Chat `json:"chat"`
		}

		newChatMessage := WebSocketMessage{
			Type: "new_chat",
			Chat: newChat,
		}

		Broadcast <- newChatMessage

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(newChat)
	}
}

func CloseChat(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		chatID, err := strconv.Atoi(r.URL.Query().Get("chat_id"))
		if err != nil {
			http.Error(w, "Invalid chat ID", http.StatusBadRequest)
			return
		}

		if err := db.Model(&models.Chat{}).Where("id = ?", chatID).Update("status", "closed").Error; err != nil {
			http.Error(w, "Failed to close chat", http.StatusInternalServerError)
			return
		}

		type WebSocketMessage struct {
			Type   string `json:"type"`
			ChatID int    `json:"chat_id"`
		}

		closeMessage := WebSocketMessage{
			Type:   "chat_closed",
			ChatID: chatID,
		}

		Broadcast <- closeMessage

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Chat closed"})
	}
}

func GetActiveChats(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var chats []models.Chat
		if err := db.Where("status = ?", "active").Find(&chats).Error; err != nil {
			http.Error(w, "Error fetching chats", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(chats)
	}
}
