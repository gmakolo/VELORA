package models

type User struct {
	ID            int
	FullName      string
	Email         string
	PasswordHash  string
	AccountNumber string
	Balance       float64
}
