package database

import (
	"database/sql"
	"errors"
	"fmt"
	"net/mail"
	"strings"

	"golang.org/x/crypto/bcrypt"
)

func AdminEmail(input string) (string, bool) {
	email := strings.ToLower(strings.TrimSpace(input))
	address, err := mail.ParseAddress(email)
	return email, len(email) <= 254 && err == nil && address.Address == email
}

func CreateAdmin(db *sql.DB, inputEmail, password string) error {
	email, ok := AdminEmail(inputEmail)
	if !ok {
		return errors.New("enter a valid admin email address")
	}
	if len(password) < 12 || len(password) > 72 {
		return errors.New("VELORA_ADMIN_PASSWORD must be between 12 and 72 bytes")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}
	result, err := db.Exec("INSERT INTO admins (email, password_hash) VALUES (?, ?) ON CONFLICT(email) DO NOTHING", email, hash)
	if err != nil {
		return fmt.Errorf("create admin: %w", err)
	}
	count, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify admin creation: %w", err)
	}
	if count == 0 {
		return errors.New("an admin with that email already exists")
	}
	return nil
}
