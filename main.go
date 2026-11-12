package main

import (
	"html/template"
	"log"
	"net/http"

	"velora/database"
	"velora/handlers"
)

func main() {

	// Connect to the database
	db, err := database.InitDB()

	if err != nil {
		log.Fatal("Database connection failed:", err)
	}

	defer db.Close()

	log.Println("VELORA database connected!")

	// Website routes
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/register", handlers.RegisterHandler(db))

	// Serve CSS and JavaScript
	http.Handle("/static/", http.StripPrefix(
		"/static/",
		http.FileServer(http.Dir("static")),
	))

	log.Println("VELORA running at http://localhost:8080")

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")

	if err != nil {
		http.Error(w, "Could not load homepage", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/login.html")

	if err != nil {
		http.Error(w, "Could not load login page", http.StatusInternalServerError)
		return
	}

	tmpl.Execute(w, nil)
}

func registerHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == http.MethodPost {
		http.Error(w, "Registration is coming in our next step!",
			http.StatusNotImplemented)
		return
	}

	tmpl, err := template.ParseFiles("templates/register.html")

	if err != nil {
		http.Error(w, "Could not load registration page",
			http.StatusInternalServerError)
		return
	}

	err = tmpl.Execute(w, nil)

	if err != nil {
		log.Println("Template error:", err)
	}
}
