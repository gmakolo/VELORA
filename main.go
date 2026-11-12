package main

import (
	"html/template"
	"log"
	"net/http"
	"os"

	"velora/database"
	"velora/handlers"

	"github.com/gorilla/sessions"
)

func main() {

	// Connect to database
	db, err := database.InitDB()

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	defer db.Close()

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
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
	}

	// Website routes
	http.HandleFunc("/", homeHandler)

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			tmpl, err := template.ParseFiles("templates/login.html")
			if err != nil {
				http.Error(w, "Could not load login page",
					http.StatusInternalServerError)
				return
			}

			tmpl.Execute(w, nil)
			return
		}

		handlers.LoginHandler(db, store)(w, r)
	})

	http.HandleFunc("/register", handlers.RegisterHandler(db))
	http.HandleFunc("/dashboard", handlers.DashboardHandler(db, store))
	http.HandleFunc("/logout", handlers.LogoutHandler(store))

	// Static files
	http.Handle("/static/", http.StripPrefix(
		"/static/",
		http.FileServer(http.Dir("static")),
	))

	log.Println("VELORA running at http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
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
