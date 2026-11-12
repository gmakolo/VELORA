package main

import (
	"fmt"
	"net/http"
)

func homeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintln(w, "Welcome to VELORA!")
	fmt.Fprintln(w, "Your money. Your future.")
}

func main() {
	http.HandleFunc("/", homeHandler)

	fmt.Println("VELORA server is running on http://localhost:8080")

	http.ListenAndServe(":8080", nil)
}
