package main

import (
	"log"
	"net/http"
	"os"

	receipt "example.com/marketplace-receipt-handoff"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = ":8080"
	}

	handler := receipt.Handler{Sender: receipt.ReceiptSender{Email: receipt.NewEmailClient(apiKey)}}
	log.Printf("receipt service listening on %s", addr)
	log.Fatal(http.ListenAndServe(addr, handler))
}
