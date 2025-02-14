package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"clinicms/models"
	"gorm.io/gorm"
)

func SendMessage(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var request struct {
			ChatID      uint   `json:"chat_id"`
			MessageText string `json:"message_text"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		userID, err := strconv.Atoi(r.Header.Get("UserID"))
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusUnauthorized)
			return
		}

		role := r.Header.Get("Role")
		if role != "admin" {
			role = "client"
		}

		var chat models.Chat
		if err := db.First(&chat, request.ChatID).Error; err != nil {
			http.Error(w, "Chat not found", http.StatusNotFound)
			return
		}

		if role == "client" && chat.ClientID != uint(userID) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		log.Printf("Received message: ChatID=%d, SenderID=%d, Role=%s, Message=%s", request.ChatID, userID, role, request.MessageText)

		message := models.Message{
			ChatID:      request.ChatID,
			SenderID:    uint(userID),
			SenderRole:  role,
			MessageText: request.MessageText,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := db.Create(&message).Error; err != nil {
			http.Error(w, "Failed to send message", http.StatusInternalServerError)
			return
		}

		log.Println("Message saved to DB:", message)

		Broadcast <- message

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(message)
	}
}

func GetChatMessages(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		chatID, err := strconv.Atoi(r.URL.Query().Get("chat_id"))
		if err != nil {
			http.Error(w, "Invalid chat ID", http.StatusBadRequest)
			return
		}

		var messages []models.Message
		if err := db.Where("chat_id = ?", chatID).Order("created_at ASC").Find(&messages).Error; err != nil {
			http.Error(w, "Failed to get messages", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(messages)
	}
}

func MarkMessagesAsRead(db *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		userID, err := strconv.Atoi(r.Header.Get("UserID"))
		if err != nil {
			http.Error(w, "Invalid user ID", http.StatusUnauthorized)
			return
		}

		role := r.Header.Get("Role")
		if role != "admin" {
			role = "client"
		}

		var request struct {
			ChatID    uint `json:"chat_id"`
			MessageID uint `json:"message_id"`
		}

		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		var chat models.Chat
		if err := db.First(&chat, request.ChatID).Error; err != nil {
			http.Error(w, "Chat not found", http.StatusNotFound)
			return
		}

		if role == "client" && chat.ClientID != uint(userID) {
			http.Error(w, "Access denied", http.StatusForbidden)
			return
		}

		var chatRead models.ChatRead
		if err := db.Where("chat_id = ? AND user_id = ? AND user_role = ?", request.ChatID, userID, role).
			First(&chatRead).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				newChatRead := models.ChatRead{
					ChatID:        request.ChatID,
					UserID:        uint(userID),
					UserRole:      role,
					LastReadMsgID: &request.MessageID,
					UpdatedAt:     time.Now(),
				}
				if err := db.Create(&newChatRead).Error; err != nil {
					http.Error(w, "Failed to create chat read record", http.StatusInternalServerError)
					return
				}
			} else {
				http.Error(w, "Database error", http.StatusInternalServerError)
				return
			}
		} else {
			if err := db.Model(&chatRead).
				Updates(map[string]interface{}{
					"last_read_message_id": request.MessageID,
					"updated_at":           time.Now(),
				}).Error; err != nil {
				http.Error(w, "Failed to update last read message", http.StatusInternalServerError)
				return
			}
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Last read message updated"})
	}
}
