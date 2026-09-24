package main

import (
	"fmt"
	"net/http"
	"os"
	"time"
)

const requestTimeout = 2 * time.Second

func main() {
	port := os.Getenv("HTTP_PORT")
	if port == "" {
		port = "8080"
	}

	client := http.Client{Timeout: requestTimeout}

	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/healthz", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: unhealthy, status", resp.StatusCode)
		os.Exit(1)
	}
}
