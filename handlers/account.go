package handlers

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/gorilla/sessions"
)

type accountActivity struct {
	Description string
	AmountKobo  int64
	Date        string
}

func (activity accountActivity) Amount() string {
	return fmt.Sprintf("₦%d.%02d", activity.AmountKobo/100, activity.AmountKobo%100)
}

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

		var name, accountNumber, avatar string
		var balance float64

		err = db.QueryRow(
			"SELECT full_name, account_number, balance, COALESCE(avatar, 'classic') FROM users LEFT JOIN user_profiles ON user_profiles.user_id = users.id WHERE users.id = ?",
			userID,
		).Scan(&name, &accountNumber, &balance, &avatar)

		if err != nil {
			http.Error(w, "Account not found", http.StatusNotFound)
			return
		}

		data := struct {
			Name             string
			AccountNumber    string
			Balance          float64
			DepositReference string
			Activity         []accountActivity
			History          bool
			Avatar           string
		}{
			Name:          name,
			AccountNumber: accountNumber,
			Balance:       balance,
			Avatar:        avatar,
		}

		data.History = r.URL.Path == "/transactions"
		limit := 5
		if data.History {
			limit = -1
		}
		rows, err := db.QueryContext(r.Context(), `
			SELECT CASE kind WHEN 'deposit' THEN 'Money added' ELSE kind END,
			       amount_kobo, substr(created_at, 1, 10)
			FROM transactions WHERE user_id = ? AND status = 'completed'
			ORDER BY created_at DESC, id DESC LIMIT ?`, userID, limit)
		if err != nil {
			http.Error(w, "Could not load account activity", http.StatusInternalServerError)
			return
		}
		defer rows.Close()
		for rows.Next() {
			var entry accountActivity
			if err := rows.Scan(&entry.Description, &entry.AmountKobo, &entry.Date); err != nil {
				http.Error(w, "Could not read account activity", http.StatusInternalServerError)
				return
			}
			data.Activity = append(data.Activity, entry)
		}
		if err := rows.Err(); err != nil {
			http.Error(w, "Could not read account activity", http.StatusInternalServerError)
			return
		}

		if reference := r.URL.Query().Get("deposit"); reference != "" {
			err := db.QueryRowContext(r.Context(),
				"SELECT reference FROM transactions WHERE reference = ? AND user_id = ? AND kind = 'deposit' AND status = 'completed'",
				reference, userID,
			).Scan(&data.DepositReference)
			if err != nil && err != sql.ErrNoRows {
				http.Error(w, "Could not load deposit confirmation", http.StatusInternalServerError)
				return
			}
		}

		tmpl, err := FormTemplate(r, store, "templates/dashboard.html")
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
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Could not end the session.", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}
