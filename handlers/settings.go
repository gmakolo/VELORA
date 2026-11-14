package handlers

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

type settingsPage struct {
	Name, Email, Account, Avatar, Error, Notice string
}

func SettingsRoutes(db *sql.DB, store *sessions.CookieStore) http.Handler {
	routes := http.NewServeMux()
	render := func(w http.ResponseWriter, r *http.Request, page settingsPage, status int) {
		tmpl, err := FormTemplate(r, store, "templates/settings.html")
		if err != nil {
			http.Error(w, "Could not load settings.", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(status)
		tmpl.Execute(w, page)
	}

	routes.HandleFunc("GET /settings", func(w http.ResponseWriter, r *http.Request) {
		id, _ := settingsUserID(r, store)
		page, err := loadSettings(r, db, id)
		if err != nil {
			http.Error(w, "Could not load your profile.", http.StatusInternalServerError)
			return
		}
		switch r.URL.Query().Get("saved") {
		case "profile":
			page.Notice = "Profile saved."
		case "password":
			page.Notice = "Password changed."
		}
		render(w, r, page, http.StatusOK)
	})
	routes.HandleFunc("POST /settings/profile", func(w http.ResponseWriter, r *http.Request) {
		id, _ := settingsUserID(r, store)
		name := strings.TrimSpace(r.PostForm.Get("name"))
		avatar := r.PostForm.Get("avatar")
		validAvatar := avatar == "classic" || avatar == "leaf" || avatar == "sun" || avatar == "mountain"
		if !utf8.ValidString(name) || name == "" || len(name) > 200 || strings.ContainsFunc(name, unicode.IsControl) || !validAvatar {
			page, err := loadSettings(r, db, id)
			if err != nil {
				http.Error(w, "Could not load your profile.", http.StatusInternalServerError)
				return
			}
			page.Error = "Enter a name of up to 200 bytes and choose a profile picture."
			render(w, r, page, http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			http.Error(w, "Could not save your profile.", http.StatusInternalServerError)
			return
		}
		defer tx.Rollback()
		if _, err = tx.ExecContext(ctx, "UPDATE users SET full_name = ? WHERE id = ?", name, id); err == nil {
			_, err = tx.ExecContext(ctx, `INSERT INTO user_profiles (user_id, avatar) VALUES (?, ?)
				ON CONFLICT(user_id) DO UPDATE SET avatar = excluded.avatar`, id, avatar)
		}
		if err != nil || tx.Commit() != nil {
			http.Error(w, "Could not save your profile.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/settings?saved=profile#profile", http.StatusSeeOther)
	})
	routes.Handle("POST /settings/password", LimitLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := settingsUserID(r, store)
		fail := func(message string, status int) {
			page, err := loadSettings(r, db, id)
			if err != nil {
				http.Error(w, "Could not load your profile.", http.StatusInternalServerError)
				return
			}
			page.Error = message
			render(w, r, page, status)
		}
		current := r.PostForm.Get("current_password")
		next := r.PostForm.Get("new_password")
		if len(next) < 12 || len(next) > 72 || next == current || next != r.PostForm.Get("confirm_password") {
			fail("Choose a different password between 12 and 72 bytes and confirm it.", http.StatusBadRequest)
			return
		}
		var hash string
		if err := db.QueryRowContext(r.Context(), "SELECT password_hash FROM users WHERE id = ?", id).Scan(&hash); err != nil {
			http.Error(w, "Could not load your credentials.", http.StatusInternalServerError)
			return
		}
		if len(current) > 72 || bcrypt.CompareHashAndPassword([]byte(hash), []byte(current)) != nil {
			fail("Your current password is incorrect.", http.StatusUnauthorized)
			return
		}
		replacement, err := bcrypt.GenerateFromPassword([]byte(next), bcrypt.DefaultCost)
		if err != nil {
			http.Error(w, "Could not hash the new password.", http.StatusInternalServerError)
			return
		}
		result, err := db.ExecContext(r.Context(), "UPDATE users SET password_hash = ? WHERE id = ? AND password_hash = ?", string(replacement), id, hash)
		if err != nil {
			http.Error(w, "Could not update your password.", http.StatusInternalServerError)
			return
		}
		count, err := result.RowsAffected()
		if err != nil || count != 1 {
			fail("Your credentials changed. Re-enter your current password.", http.StatusConflict)
			return
		}
		http.Redirect(w, r, "/settings?saved=password#password", http.StatusSeeOther)
	})))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := settingsUserID(r, store); !ok {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		routes.ServeHTTP(w, r)
	})
}

func settingsUserID(r *http.Request, store *sessions.CookieStore) (int, bool) {
	session, err := store.Get(r, "velora-session")
	id, ok := session.Values["user_id"].(int)
	return id, ok && err == nil
}

func loadSettings(r *http.Request, db *sql.DB, id int) (settingsPage, error) {
	var page settingsPage
	err := db.QueryRowContext(r.Context(), `
		SELECT full_name, email, account_number, COALESCE(avatar, 'classic')
		FROM users LEFT JOIN user_profiles ON user_profiles.user_id = users.id WHERE users.id = ?`, id).
		Scan(&page.Name, &page.Email, &page.Account, &page.Avatar)
	return page, err
}
