package handlers

import (
	"database/sql"
	"html/template"
	"net/http"

	"github.com/gorilla/sessions"
)

func DashboardHandler(db *sql.DB, store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "velora-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		userID, ok := session.Values["user_id"].(int)
		if !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		var name, accountNumber string
		var balance float64

		err = db.QueryRow(
			"SELECT full_name, account_number, balance FROM users WHERE id = ?",
			userID,
		).Scan(&name, &accountNumber, &balance)

		if err != nil {
			http.Error(w, "Account not found", http.StatusNotFound)
			return
		}

		data := struct {
			Name          string
			AccountNumber string
			Balance       float64
		}{
			Name:          name,
			AccountNumber: accountNumber,
			Balance:       balance,
		}

		tmpl, err := template.ParseFiles("templates/dashboard.html")
		if err != nil {
			http.Error(w, "Could not load dashboard", http.StatusInternalServerError)
			return
		}

		tmpl.Execute(w, data)
	}
}

func LogoutHandler(store *sessions.CookieStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, "velora-session")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		session.Options.MaxAge = -1
		session.Save(r, w)

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
