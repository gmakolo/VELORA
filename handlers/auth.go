package handlers

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
	"net/mail"
	"strings"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

func RegisterHandler(db *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Invalid form", http.StatusBadRequest)
			return
		}

		fullName := strings.TrimSpace(r.PostForm.Get("fullname"))
		email := strings.TrimSpace(r.PostForm.Get("email"))
		password := r.PostForm.Get("password")
		confirmPassword := r.PostForm.Get("confirm_password")

		if fullName == "" || email == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		if password != confirmPassword {
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
			return
		}
		address, err := mail.ParseAddress(email)
		if len(fullName) > 200 || len(email) > 254 || err != nil || address.Address != email || len(password) > 72 {
			http.Error(w, "Check your name, email address, and password length.", http.StatusBadRequest)
			return
		}

		if len(password) < 8 {
			http.Error(w, "Password must contain at least 8 characters",
				http.StatusBadRequest)
			return
		}

		var existingEmail string

		err = db.QueryRow(
			"SELECT email FROM users WHERE email = ?",
			email,
		).Scan(&existingEmail)

		if err == nil {
			http.Error(w, "Email already registered", http.StatusConflict)
			return
		}

		if err != sql.ErrNoRows {
			http.Error(w, "Database error", http.StatusInternalServerError)
			return
		}

		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(password),
			bcrypt.DefaultCost,
		)

		if err != nil {
			http.Error(w, "Could not secure password",
				http.StatusInternalServerError)
			return
		}

		accountNumber := fmt.Sprintf("%010d", rand.Int63n(10000000000))

		_, err = db.Exec(`
			INSERT INTO users
			(full_name, email, password_hash, account_number, balance)
			VALUES (?, ?, ?, ?, ?)`,
			fullName,
			email,
			string(hashedPassword),
			accountNumber,
			0,
		)

		if err != nil {
			http.Error(w, "Could not create account",
				http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "Welcome to VELORA, %s! Your demo account number is %s.",
			fullName, accountNumber)
	}
}

func LoginPage(store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		renderLogin(w, r, store, http.StatusOK, "", "")
	}
}

func renderLogin(w http.ResponseWriter, r *http.Request, store *sessions.CookieStore, status int, email, message string) {
	tmpl, err := FormTemplate(r, store, "templates/login.html")
	if err != nil {
		http.Error(w, "Could not load login page", http.StatusInternalServerError)
		return
	}
	session, _ := store.Get(r, "velora-session")
	_, signedIn := session.Values["user_id"].(int)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	tmpl.Execute(w, struct {
		Email    string
		Message  string
		SignedIn bool
	}{email, message, signedIn})
}

func LoginHandler(db *sql.DB, store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email := strings.TrimSpace(r.PostFormValue("email"))
		password := r.PostFormValue("password")
		if email == "" || len(email) > 254 || password == "" || len(password) > 72 {
			renderLogin(w, r, store, http.StatusUnauthorized, "", "Enter your email address and password.")
			return
		}

		var userID int
		var passwordHash string

		err := db.QueryRow(
			"SELECT id, password_hash FROM users WHERE email = ?",
			email,
		).Scan(&userID, &passwordHash)

		if err != nil || bcrypt.CompareHashAndPassword(
			[]byte(passwordHash), []byte(password),
		) != nil {
			renderLogin(w, r, store, http.StatusUnauthorized, email, "The email or password is incorrect.")
			return
		}

		session := sessions.NewSession(store, "velora-session")
		session.Options = store.Options
		session.Values["user_id"] = userID

		err = session.Save(r, w)
		if err != nil {
			http.Error(w, "Could not save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}
