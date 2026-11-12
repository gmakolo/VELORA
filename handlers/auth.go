package handlers

import (
	"database/sql"
	"fmt"
	"math/rand"
	"net/http"
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

		fullName := strings.TrimSpace(r.FormValue("fullname"))
		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")
		confirmPassword := r.FormValue("confirm_password")

		if fullName == "" || email == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		if password != confirmPassword {
			http.Error(w, "Passwords do not match", http.StatusBadRequest)
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

		fmt.Fprintf(w, "Welcome to VELORA, %s! Your demo account number is %s.",
			fullName, accountNumber)
	}
}

func LoginHandler(db *sql.DB, store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		email := strings.TrimSpace(r.FormValue("email"))
		password := r.FormValue("password")

		var userID int
		var passwordHash string

		err := db.QueryRow(
			"SELECT id, password_hash FROM users WHERE email = ?",
			email,
		).Scan(&userID, &passwordHash)

		if err != nil || bcrypt.CompareHashAndPassword(
			[]byte(passwordHash), []byte(password),
		) != nil {
			http.Error(w, "Invalid email or password", http.StatusUnauthorized)
			return
		}

		session, err := store.Get(r, "velora-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		session.Values["user_id"] = userID

		err = session.Save(r, w)
		if err != nil {
			http.Error(w, "Could not save session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	}
}
