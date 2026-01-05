package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/server"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	log.Printf("listening on %s", addr)
	if err := http.ListenAndServe(addr, server.NewMux()); err != nil {
		log.Fatal(err)
	}
}
