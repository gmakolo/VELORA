package handlers

import (
	"crypto/rand"
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gorilla/sessions"
)

var amountPattern = regexp.MustCompile(`^[0-9]{1,9}(\.[0-9]{1,2})?$`)

func DepositHandler(db *sql.DB, store *sessions.CookieStore) http.HandlerFunc {
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

		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Could not read the deposit form.", http.StatusBadRequest)
			return
		}
		amount := strings.TrimSpace(r.PostForm.Get("amount"))
		if !amountPattern.MatchString(amount) {
			http.Error(w, "Enter an amount from ₦0.01 to ₦999,999,999.99, with up to two decimal places and no commas.", http.StatusBadRequest)
			return
		}
		naira, fraction, _ := strings.Cut(amount, ".")
		whole, _ := strconv.ParseInt(naira, 10, 64)
		kobo, _ := strconv.ParseInt((fraction + "00")[:2], 10, 64)
		amountKobo := whole*100 + kobo
		if amountKobo == 0 {
			http.Error(w, "Enter an amount greater than zero.", http.StatusBadRequest)
			return
		}

		ctx := r.Context()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			http.Error(w, "Could not start the deposit.", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()

		result, err := tx.ExecContext(ctx,
			"UPDATE users SET balance = (ROUND(balance * 100) + ?) / 100.0 WHERE id = ?",
			amountKobo, userID,
		)
		if err != nil {
			http.Error(w, "Could not update your balance.", http.StatusInternalServerError)
			return
		}
		updated, err := result.RowsAffected()
		if err != nil {
			http.Error(w, "Could not verify the balance update.", http.StatusInternalServerError)
			return
		}
		if updated == 0 {
			http.Error(w, "Account not found.", http.StatusNotFound)
			return
		}

		reference := rand.Text()
		_, err = tx.ExecContext(ctx, `
			INSERT INTO transactions (user_id, reference, kind, amount_kobo, status)
			VALUES (?, ?, 'deposit', ?, 'completed')`,
			userID, reference, amountKobo,
		)
		if err != nil {
			http.Error(w, "Could not record the deposit.", http.StatusInternalServerError)
			return
		}
		if err := tx.Commit(); err != nil {
			http.Error(w, "Could not complete the deposit.", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/dashboard?deposit="+reference, http.StatusSeeOther)
	}
}
