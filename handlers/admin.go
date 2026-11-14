package handlers

import (
	"database/sql"

	"net/http"

	"velora/database"

	"github.com/gorilla/sessions"
	"golang.org/x/crypto/bcrypt"
)

const adminSessionName = "velora-admin"

func AdminRoutes(db *sql.DB, store *sessions.CookieStore) http.Handler {
	routes := http.NewServeMux()
	protected := http.NewServeMux()

	showLogin := func(w http.ResponseWriter, r *http.Request, status int, message string) {
		renderAdminPage(w, r, store, "templates/admin_login.html", status, message)
	}
	routes.HandleFunc("GET /admin/login", func(w http.ResponseWriter, r *http.Request) {
		showLogin(w, r, http.StatusOK, "")
	})
	routes.Handle("POST /admin/login", LimitLogin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			showLogin(w, r, http.StatusBadRequest, "Could not read the sign-in form.")
			return
		}
		email, ok := database.AdminEmail(r.PostForm.Get("email"))
		password := r.PostForm.Get("password")
		if !ok || len(password) < 12 || len(password) > 72 {
			showLogin(w, r, http.StatusUnauthorized, "Invalid admin email or password.")
			return
		}

		var id int
		var hash string
		err := db.QueryRowContext(r.Context(), "SELECT id, password_hash FROM admins WHERE email = ?", email).Scan(&id, &hash)
		if err != nil && err != sql.ErrNoRows {
			http.Error(w, "Could not sign in.", http.StatusInternalServerError)
			return
		}
		if err == sql.ErrNoRows || bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
			showLogin(w, r, http.StatusUnauthorized, "Invalid admin email or password.")
			return
		}

		session := sessions.NewSession(store, adminSessionName)
		options := *store.Options
		options.Path = "/admin"
		session.Options = &options
		session.Values["admin_id"] = id
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Could not save the admin session.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/", http.StatusSeeOther)
	})))

	protected.HandleFunc("GET /admin/{$}", func(w http.ResponseWriter, r *http.Request) {
		renderAdminPage(w, r, store, "templates/admin_dashboard.html", http.StatusOK, nil)
	})
	protected.HandleFunc("POST /admin/logout", func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, adminSessionName)
		session.Options.Path = "/admin"
		session.Options.MaxAge = -1
		if err := session.Save(r, w); err != nil {
			http.Error(w, "Could not end the admin session.", http.StatusInternalServerError)
			return
		}
		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	})
	routes.Handle("/admin/", requireAdmin(db, store, protected))

	return http.NewCrossOriginProtection().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		routes.ServeHTTP(w, r)
	}))
}

func requireAdmin(db *sql.DB, store *sessions.CookieStore, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, err := store.Get(r, adminSessionName)
		id, ok := session.Values["admin_id"].(int)
		if err != nil || !ok {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		var activeID int
		err = db.QueryRowContext(r.Context(), "SELECT id FROM admins WHERE id = ?", id).Scan(&activeID)
		if err == sql.ErrNoRows {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		if err != nil {
			http.Error(w, "Could not verify admin access.", http.StatusInternalServerError)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func renderAdminPage(w http.ResponseWriter, r *http.Request, store *sessions.CookieStore, path string, status int, data any) {
	tmpl, err := FormTemplate(r, store, path)
	if err != nil {
		http.Error(w, "Could not load the admin page.", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	tmpl.Execute(w, data)
}
