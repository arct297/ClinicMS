package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"

	"clinicms/handlers"
	"clinicms/logger"
	"clinicms/models"
	"clinicms/tools"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

var clients = make(map[*websocket.Conn]bool)
var clientsMutex sync.Mutex
var broadcast = make(chan interface{})

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("WebSocket upgrade error:", err)
		return
	}
	defer ws.Close()

	clientsMutex.Lock()
	clients[ws] = true
	clientsMutex.Unlock()

	for {
		var msg models.Message
		err := ws.ReadJSON(&msg)
		if err != nil {
			log.Println("WebSocket read error:", err)
			clientsMutex.Lock()
			delete(clients, ws)
			clientsMutex.Unlock()
			break
		}

		saveMessageToDB(msg)
		broadcast <- msg
	}
}

func saveMessageToDB(msg models.Message) {
	db := tools.DB
	newMessage := models.Message{
		ChatID:      msg.ChatID,
		SenderID:    msg.SenderID,
		SenderRole:  msg.SenderRole,
		MessageText: msg.MessageText,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := db.Create(&newMessage).Error; err != nil {
		log.Println("Error saving message to DB:", err)
	}
}

func handleMessages() {
	for {
		msg := <-broadcast
		log.Println("Broadcasting message:", msg)

		clientsMutex.Lock()
		for client := range clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Println("WebSocket send error:", err)
				client.Close()
				delete(clients, client)
			} else {
				log.Println("Message sent to client successfully")
			}
		}
		clientsMutex.Unlock()
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	tools.InitDatabaseClient()

	if err := logger.InitLogger(); err != nil {
		log.Fatalf("Logger initialization failed: %v", err)
	}

	sqlDB, err := tools.DB.DB()
	if err != nil {
		log.Fatal("Failed to get sql.DB from GORM DB:", err)
	}
	defer sqlDB.Close()

	go func() {
		for {
			msg := <-handlers.Broadcast
			clientsMutex.Lock()
			for client := range clients {
				err := client.WriteJSON(msg)
				if err != nil {
					log.Println("WebSocket send error:", err)
					client.Close()
					delete(clients, client)
				}
			}
			clientsMutex.Unlock()
		}
	}()

	r := mux.NewRouter()

	// WebSocket route
	r.HandleFunc("/ws", handleWebSocket)
	go handleMessages()

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// HTML pages
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "login.html"))
	}).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "index.html"))
	}).Methods("GET")
	r.HandleFunc("/signup", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "signup.html"))
	}).Methods("GET")
	r.HandleFunc("/patients", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "patients.html"))
	}).Methods("GET")

	// Public API
	publicRoutes := r.PathPrefix("/api").Subrouter()
	publicRoutes.HandleFunc("/register", handlers.RegisterUser).Methods("POST")
	publicRoutes.HandleFunc("/login", handlers.LoginUser).Methods("POST")
	publicRoutes.HandleFunc("/logout", handlers.LogoutUser).Methods("GET")
	publicRoutes.HandleFunc("/confirm", handlers.ConfirmEmail).Methods("GET")

	// Protected API
	protectedRoutes := r.PathPrefix("/api").Subrouter()
	protectedRoutes.Use(tools.JWTAuthMiddleware)
	protectedRoutes.HandleFunc("/patients", handlers.GetPatientsList).Methods("GET")
	protectedRoutes.HandleFunc("/patients/{id}", handlers.GetPatientByID).Methods("GET")
	protectedRoutes.HandleFunc("/patients", handlers.CreatePatient).Methods("POST")
	protectedRoutes.HandleFunc("/patients/{id}", handlers.UpdatePatient).Methods("PUT")
	protectedRoutes.HandleFunc("/patients/{id}", handlers.DeletePatient).Methods("DELETE")

	// Chat routes
	protectedRoutes.HandleFunc("/ts-chat", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "chat.html"))
	}).Methods("GET")

	protectedRoutes.HandleFunc("/create_chat", handlers.CreateChat(tools.DB)).Methods("POST")
	protectedRoutes.HandleFunc("/close_chat", handlers.CloseChat(tools.DB)).Methods("POST")
	protectedRoutes.HandleFunc("/send_message", handlers.SendMessage(tools.DB)).Methods("POST")
	protectedRoutes.HandleFunc("/get_messages", handlers.GetChatMessages(tools.DB)).Methods("GET")
	protectedRoutes.HandleFunc("/mark_as_read", handlers.MarkMessagesAsRead(tools.DB)).Methods("POST")

	// Admin API
	adminRoutes := r.PathPrefix("/admin").Subrouter()
	adminRoutes.Use(tools.JWTAuthMiddleware)
	adminRoutes.Use(tools.RoleMiddleware("admin"))
	adminRoutes.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "admin.html"))
	}).Methods("GET")
	adminRoutes.HandleFunc("/ts-chat", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "admin-chat.html"))
	}).Methods("GET")
	adminRoutes.HandleFunc("/mailing", handlers.MakeMailing).Methods("POST")
	adminRoutes.HandleFunc("/patients", handlers.GetPatientsList).Methods("GET")

	// Admin chat
	adminRoutes.HandleFunc("/get_active_chats", handlers.GetActiveChats(tools.DB)).Methods("GET")
	adminRoutes.HandleFunc("/close_chat", handlers.CloseChat(tools.DB)).Methods("POST")

	// Doctor API
	r.HandleFunc("/doctors", handlers.GetDoctorsList).Methods("GET")

	// Server setup
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Server is running on port 8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	<-quit
	log.Println("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
