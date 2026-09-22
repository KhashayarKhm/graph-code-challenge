package main

import "graph-code-challenge/internal/delivery/httpserver"

func main() {
	s := httpserver.New()
	s.Setup()
	s.Serve(":8080")
}
