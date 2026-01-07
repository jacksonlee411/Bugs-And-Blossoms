package main

import (
	"log"
	"net/http"
	"os"

	"github.com/jacksonlee411/Bugs-And-Blossoms/internal/superadmin"
)

func main() {
	addr := os.Getenv("SUPERADMIN_HTTP_ADDR")
	if addr == "" {
		addr = ":8081"
	}

	log.Printf("superadmin listening on %s", addr)
	if err := http.ListenAndServe(addr, superadmin.MustNewHandler()); err != nil {
		log.Fatal(err)
	}
}
