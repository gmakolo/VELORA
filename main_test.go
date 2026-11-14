package main

import (
	"crypto/rand"
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"velora/database"

	"github.com/gorilla/sessions"
)

type browser struct {
	handler http.Handler
	cookies map[string]*http.Cookie
	token   string
}

var tokenPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

func (b *browser) request(t *testing.T, method, path string, values url.Values, want int) string {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range b.cookies {
		req.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	b.handler.ServeHTTP(response, req)
	for _, cookie := range response.Result().Cookies() {
		if cookie.MaxAge < 0 {
			delete(b.cookies, cookie.Name)
		} else {
			b.cookies[cookie.Name] = cookie
		}
	}
	body := response.Body.String()
	if response.Code != want {
		t.Fatalf("%s %s returned %d, want %d: %s", method, path, response.Code, want, body)
	}
	if match := tokenPattern.FindStringSubmatch(body); match != nil {
		b.token = match[1]
	}
	return body
}

func (b *browser) post(t *testing.T, path string, values url.Values, want int) {
	t.Helper()
	values.Set("csrf_token", b.token)
	b.request(t, http.MethodPost, path, values, want)
}

func TestBankingRoutes(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "test.db"))
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	const password = "ExamplePass123"
	if err := database.CreateAdmin(db, "admin@example.test", password); err != nil {
		t.Fatal(err)
	}

	server := appRoutes(db, sessions.NewCookieStore([]byte(rand.Text()+rand.Text())))
	customer := &browser{handler: server, cookies: make(map[string]*http.Cookie)}
	customer.request(t, "GET", "/dashboard", nil, 303)
	customer.request(t, "GET", "/transactions", nil, 303)
	customer.request(t, "GET", "/register", nil, 200)
	credentials := url.Values{"email": {"customer@example.test"}, "password": {password}}
	registration := url.Values{"email": {credentials.Get("email")}, "password": {password}, "confirm_password": {password}, "fullname": {"Demo <Account>"}}
	customer.request(t, "POST", "/register", registration, 403)
	customer.post(t, "/register", registration, 200)
	customer.post(t, "/register", registration, 409)
	login := customer.request(t, "GET", "/login", nil, 200)
	if !strings.Contains(login, "Welcome back") || strings.Contains(login, "Switch Account") {
		t.Fatal("unexpected signed-out login page")
	}
	wrongLogin := url.Values{"email": {"' OR 1=1 --"}, "password": {password}, "csrf_token": {customer.token}}
	rejected := customer.request(t, "POST", "/login", wrongLogin, 401)
	if !strings.Contains(rejected, "The email or password is incorrect.") || strings.Contains(rejected, password) {
		t.Fatal("login form did not display a safe error")
	}
	customer.post(t, "/login", credentials, 303)

	if body := customer.request(t, "GET", "/dashboard", nil, 200); !strings.Contains(body, "Demo &lt;Account&gt;") {
		t.Fatal("dashboard did not escape customer name")
	}
	customer.request(t, "POST", "/deposit", url.Values{"amount": {"1"}}, 403)
	for _, amount := range []string{"0", "-1", "1e3", "1.001"} {
		customer.post(t, "/deposit", url.Values{"amount": {amount}}, 400)
	}
	customer.post(t, "/deposit", url.Values{"amount": {"12.34"}}, 303)
	var reference string
	var amount int
	var balance float64
	err = db.QueryRow("SELECT reference, amount_kobo, balance FROM transactions JOIN users ON users.id = transactions.user_id").Scan(&reference, &amount, &balance)
	if err != nil || reference == "" || amount != 1234 || balance != 12.34 {
		t.Fatalf("deposit was not saved correctly: amount=%d balance=%v err=%v", amount, balance, err)
	}
	if body := customer.request(t, "GET", "/transactions", nil, 200); !strings.Contains(body, "Money added") || !strings.Contains(body, "₦12.34") {
		t.Fatal("transaction history is missing the customer's deposit")
	}
	other := &browser{handler: server, cookies: make(map[string]*http.Cookie)}
	other.request(t, "GET", "/register", nil, 200)
	other.post(t, "/register", url.Values{"email": {"other@example.test"}, "password": {password}, "confirm_password": {password}, "fullname": {"Other Customer"}}, 200)
	other.post(t, "/login", url.Values{"email": {"other@example.test"}, "password": {password}}, 303)
	if body := other.request(t, "GET", "/transactions", nil, 200); !strings.Contains(body, "Your transactions will appear here.") || strings.Contains(body, "₦12.34") {
		t.Fatal("transaction history revealed another customer's deposit")
	}
	if body := other.request(t, "GET", "/dashboard?deposit="+reference, nil, 200); strings.Contains(body, reference) || strings.Contains(body, "12.34") {
		t.Fatal("dashboard revealed another customer's deposit")
	}
	if _, err := db.Exec("UPDATE users SET balance = -1"); err == nil {
		t.Fatal("SQLite accepted a negative balance")
	}
	if _, err := db.Exec("INSERT INTO transactions (user_id, reference, kind, amount_kobo, status) VALUES (999, 'orphan', 'deposit', 1, 'completed')"); err == nil {
		t.Fatal("SQLite accepted a transaction for a nonexistent user")
	}

	// Force the journal write to fail and verify the balance update rolls back.
	if _, err := db.Exec("CREATE TRIGGER reject_test_deposit BEFORE INSERT ON transactions BEGIN SELECT RAISE(ABORT, 'test failure'); END"); err != nil {
		t.Fatal(err)
	}
	customer.post(t, "/deposit", url.Values{"amount": {"1"}}, 500)
	if err := db.QueryRow("SELECT balance FROM users WHERE email = ?", credentials.Get("email")).Scan(&balance); err != nil || balance != 12.34 {
		t.Fatalf("failed deposit changed balance: %v, %v", balance, err)
	}

	customer.request(t, "GET", "/admin/", nil, 303)
	customer.request(t, "GET", "/admin/login", nil, 200)
	customer.post(t, "/admin/login", credentials, 401)
	admin := &browser{handler: server, cookies: make(map[string]*http.Cookie)}
	admin.request(t, "GET", "/admin/login", nil, 200)
	admin.post(t, "/admin/login", url.Values{"email": {"admin@example.test"}, "password": {password}}, 303)
	admin.request(t, "GET", "/admin/", nil, 200)
	admin.request(t, "GET", "/dashboard", nil, 303)
	admin.post(t, "/admin/logout", url.Values{}, 303)
	admin.request(t, "GET", "/admin/", nil, 303)

	customer.request(t, "GET", "/logout", nil, 405)
	if body := customer.request(t, "GET", "/login", nil, 200); !strings.Contains(body, "Switch Account") {
		t.Fatal("signed-in login page did not offer account switching")
	}
	customer.post(t, "/logout", url.Values{}, 303)
	customer.request(t, "GET", "/dashboard", nil, 303)
	for i := 0; i < 7; i++ {
		customer.post(t, "/login", url.Values{}, 401)
	}
	customer.post(t, "/login", credentials, 429)
}

func TestExistingBalanceSurvivesSchemaUpdate(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "existing.db"))
	legacy, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = legacy.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, balance REAL NOT NULL);
		INSERT INTO users (id, balance) VALUES (1, 42.5);
	`)
	legacy.Close()
	if err != nil {
		t.Fatal(err)
	}
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var balance float64
	if err := db.QueryRow("SELECT balance FROM users WHERE id = 1").Scan(&balance); err != nil || balance != 42.5 {
		t.Fatalf("schema update changed existing balance: %v, %v", balance, err)
	}
}

func TestProfileSettings(t *testing.T) {
	path := filepath.ToSlash(filepath.Join(t.TempDir(), "settings.db"))
	db, err := database.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	server := appRoutes(db, sessions.NewCookieStore([]byte(rand.Text()+rand.Text())))
	user := &browser{handler: server, cookies: make(map[string]*http.Cookie)}
	user.request(t, "GET", "/settings", nil, 303)
	user.request(t, "GET", "/register", nil, 200)
	const email = "profile@example.test"
	const original = "OriginalPassword123"
	const replacement = "ReplacementPassword123"
	user.post(t, "/settings/profile", url.Values{"name": {"Blocked"}, "avatar": {"leaf"}}, 303)
	user.post(t, "/register", url.Values{"email": {email}, "password": {original}, "confirm_password": {original}, "fullname": {"Before"}}, 200)
	user.post(t, "/login", url.Values{"email": {email}, "password": {original}}, 303)
	settings := user.request(t, "GET", "/settings", nil, 200)
	if !strings.Contains(settings, email) || !strings.Contains(settings, "classic.svg") {
		t.Fatal("settings did not show the current profile")
	}
	user.request(t, "POST", "/settings/profile", url.Values{"name": {"Blocked"}, "avatar": {"leaf"}}, 403)
	user.post(t, "/settings/profile", url.Values{"name": {"Invalid"}, "avatar": {"../invalid"}}, 400)
	user.post(t, "/settings/profile", url.Values{"name": {"After"}, "avatar": {"leaf"}, "email": {"other@example.test"}, "account_number": {"changed"}}, 303)
	if page := user.request(t, "GET", "/dashboard", nil, 200); !strings.Contains(page, "Hi, After!") || !strings.Contains(page, "leaf.svg") {
		t.Fatal("dashboard missed the saved profile")
	}
	var savedEmail, account string
	if err := db.QueryRow("SELECT email, account_number FROM users").Scan(&savedEmail, &account); err != nil || savedEmail != email || len(account) != 10 {
		t.Fatalf("read-only profile information changed: %v", err)
	}
	user.post(t, "/settings/password", url.Values{"current_password": {"wrong"}, "new_password": {replacement}, "confirm_password": {replacement}}, 401)
	user.post(t, "/settings/password", url.Values{"current_password": {original}, "new_password": {replacement}, "confirm_password": {replacement}}, 303)
	user.post(t, "/logout", url.Values{}, 303)
	user.post(t, "/login", url.Values{"email": {email}, "password": {original}}, 401)
	user.post(t, "/login", url.Values{"email": {email}, "password": {replacement}}, 303)
}
