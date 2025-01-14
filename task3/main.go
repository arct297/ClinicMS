package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"golang.org/x/time/rate"

	_ "github.com/lib/pq"

	"task3/handlers"
	"task3/logger"
	"task3/tools"
)

// Define a global rate limiter
var limiter = rate.NewLimiter(1, 3) // 1 request per second with a burst of 3 requests

// Rate-limiting middleware
func rateLimiterMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !limiter.Allow() {
			// If rate limit is exceeded, respond with 429 Too Many Requests
			w.Header().Set("Retry-After", time.Now().Add(limiter.Reserve().Delay()).Format(time.RFC1123))
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func main() {
	// Initialize database client
	tools.InitDatabaseClient()

	// Initialize logger
	if err := logger.InitLogger(); err != nil {
		log.Fatalf("Logger initialization failed: %v", err)
	}

	// Get SQL database instance from GORM
	sqlDB, err := tools.DB.DB()
	if err != nil {
		log.Fatal("Failed to get sql.DB from GORM DB:", err)
	}
	defer sqlDB.Close()

	// Initialize router
	r := mux.NewRouter()

	// Serve static files like HTML, CSS, JS
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	// Return index.html on root path
	r.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "admin.html"))
	}).Methods("GET")
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "login.html"))
	}).Methods("GET")
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "index.html"))
	}).Methods("GET")
	r.HandleFunc("/patients", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join("static", "patients.html"))
	}).Methods("GET")

	// Patient API routes
	r.HandleFunc("/patients", handlers.CreatePatient).Methods("POST")        // Create patient
	r.HandleFunc("/patients/{id}", handlers.GetPatientByID).Methods("GET")   // Get patient by ID
	r.HandleFunc("/patients", handlers.GetPatientsList).Methods("GET")       // Get patients list
	r.HandleFunc("/patients/{id}", handlers.UpdatePatient).Methods("PUT")    // Update patient by ID
	r.HandleFunc("/patients/{id}", handlers.DeletePatient).Methods("DELETE") // Delete patient by ID

	// Doctor API route
	r.HandleFunc("/doctors", handlers.GetDoctorsList).Methods("GET")

	// Mailing API route
	r.HandleFunc("/mailing", handlers.MakeMailing).Methods("POST")

	// Apply rate-limiting middleware
	rateLimitedRouter := rateLimiterMiddleware(r)

	// Create HTTP server
	srv := &http.Server{
		Addr:    ":8080",
		Handler: rateLimitedRouter,
	}

	// Channel to listen for OS signals
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Println("Server is running on port 8080...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("ListenAndServe(): %v", err)
		}
	}()

	// Wait for termination signal
	<-quit
	log.Println("Server is shutting down...")

	// Graceful shutdown with a timeout of 30 seconds
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exiting")
}
