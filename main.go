package main

import (
	"errors"
	"fmt"
	"net/http"

	health "hello-go/internal"
)

func main() {
	fmt.Println("Starting application...")

	healthController := health.NewController()
	res, err := divide(10, 0)
	if err != nil {
		fmt.Println("Error:", err)
	}

	fmt.Println(res[0] + res[0])

	http.HandleFunc("/health", healthController.HealthHandler)
	http.ListenAndServe(":8080", nil)
}

func divide(a, b int) ([]int, error) {
	if b == 0 {
		return nil, errors.New("division by zero")
	}
	return []int{a / b}, nil
}
