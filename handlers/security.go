package handlers

import (
	"crypto/rand"
	"crypto/subtle"
	"html/template"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/gorilla/sessions"
)

const csrfSession = "velora-csrf"

func ProtectForms(store *sessions.CookieStore, next http.Handler) http.Handler {
	return http.NewCrossOriginProtection().Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		session, _ := store.Get(r, csrfSession)
		token, _ := session.Values["token"].(string)
		switch r.Method {
		case http.MethodGet, http.MethodHead:
			if token == "" {
				session.Options = store.Options
				session.Values["token"] = rand.Text()
				if err := session.Save(r, w); err != nil {
					http.Error(w, "Could not prepare the form.", http.StatusInternalServerError)
					return
				}
			}
		default:
			r.Body = http.MaxBytesReader(w, r.Body, 8192)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "Invalid form.", http.StatusBadRequest)
				return
			}
			if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(r.PostForm.Get("csrf_token"))) != 1 {
				http.Error(w, "Form expired or invalid. Reload the page and try again.", http.StatusForbidden)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	}))
}

func FormTemplate(r *http.Request, store *sessions.CookieStore, path string) (*template.Template, error) {
	return template.New(filepath.Base(path)).Funcs(template.FuncMap{
		"csrfToken": func() string {
			session, _ := store.Get(r, csrfSession)
			token, _ := session.Values["token"].(string)
			return token
		},
	}).ParseFiles(path)
}

func LimitLogin(next http.Handler) http.Handler {
	type window struct {
		until time.Time
		count int
	}
	var mu sync.Mutex
	attempts := make(map[string]window)
	var cleanup time.Time
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			host = r.RemoteAddr
		}
		now := time.Now()
		mu.Lock()
		if now.After(cleanup) {
			for key, entry := range attempts {
				if !now.Before(entry.until) {
					delete(attempts, key)
				}
			}
			cleanup = now.Add(time.Minute)
		}
		entry := attempts[host]
		if !now.Before(entry.until) {
			entry = window{until: now.Add(time.Minute)}
		}
		allowed := entry.count < 10 && (len(attempts) < 10000 || attempts[host].count > 0)
		if allowed {
			entry.count++
			attempts[host] = entry
		}
		mu.Unlock()
		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many sign-in attempts. Try again in a minute.", http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}
