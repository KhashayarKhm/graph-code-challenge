package main

import "graph-code-challenge/internal/delivery/httpserver"

func main() {
	s := &httpserver.Server{}
	s.Setup()
	s.Serve(":8080")
}
