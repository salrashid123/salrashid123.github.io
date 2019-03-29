package main

import (
	"fmt"
	"log"
	"net/http"
)

func handler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Hello Ben, Fernando, Vic")
}

func hchandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "ok")
}

func main() {
	http.HandleFunc("/", handler)
	http.HandleFunc("/_ah/health", hchandler)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
