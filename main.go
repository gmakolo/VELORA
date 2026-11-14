package main

import (
	"database/sql"
	"flag"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"velora/database"
	"velora/handlers"

	"github.com/gorilla/sessions"
)

func main() {
	createAdmin := flag.String("create-admin", "", "Create an admin with this email and exit")
	flag.Parse()

	// Connect to database
	db, err := database.InitDB()

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	defer db.Close()

	if *createAdmin != "" {
		if err := database.CreateAdmin(db, *createAdmin, os.Getenv("VELORA_ADMIN_PASSWORD")); err != nil {
			log.Fatal(err)
		}
		log.Println("Admin account created.")
		return
	}

	log.Println("VELORA database connected!")

	// Configure sessions
	sessionKey := os.Getenv("VELORA_SESSION_KEY")

	if len(sessionKey) < 32 {
		log.Fatal("VELORA_SESSION_KEY must be at least 32 characters")
	}

	store := sessions.NewCookieStore([]byte(sessionKey))

	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400,
		HttpOnly: true,
		Secure:   os.Getenv("VELORA_SECURE_COOKIES") == "true",
		SameSite: http.SameSiteLaxMode,
	}
	store.MaxAge(store.Options.MaxAge)

	server := &http.Server{
		Addr:              ":8080",
		Handler:           appRoutes(db, store),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       time.Minute,
	}
	log.Println("VELORA running at http://localhost:8080")
	log.Fatal(server.ListenAndServe())
}

func appRoutes(db *sql.DB, store *sessions.CookieStore) http.Handler {
	mux := http.NewServeMux()
	private := http.NewServeMux()
	private.HandleFunc("GET /{$}", homeHandler)

	private.HandleFunc("GET /login", handlers.LoginPage(store))
	private.Handle("POST /login", handlers.LimitLogin(handlers.LoginHandler(db, store)))

	private.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		tmpl, err := handlers.FormTemplate(r, store, "templates/register.html")
		if err != nil {
			http.Error(w, "Could not load registration page", http.StatusInternalServerError)
			return
		}

		tmpl.Execute(w, nil)
	})
	private.HandleFunc("POST /register", handlers.RegisterHandler(db))
	private.HandleFunc("GET /dashboard", handlers.DashboardHandler(db, store))
	private.HandleFunc("GET /transactions", handlers.DashboardHandler(db, store))
	settings := handlers.SettingsRoutes(db, store)
	private.Handle("/settings", settings)
	private.Handle("/settings/", settings)
	private.HandleFunc("POST /deposit", handlers.DepositHandler(db, store))
	private.HandleFunc("POST /logout", handlers.LogoutHandler(store))
	private.Handle("/admin/", handlers.AdminRoutes(db, store))

	// Static files
	mux.Handle("/static/", http.StripPrefix(
		"/static/",
		http.FileServer(http.Dir("static")),
	))

	mux.Handle("/", handlers.ProtectForms(store, private))
	return mux
}

// Homepage handler
func homeHandler(w http.ResponseWriter, r *http.Request) {

	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	tmpl, err := template.ParseFiles("templates/index.html")

	if err != nil {
		http.Error(w, "Could not load homepage",
			http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}
