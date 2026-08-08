package main

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
)

type Client struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var (
	mu      = sync.Mutex{}
	clients = make(map[int]Client)
	nextID  = 1
)

func clientHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		mu.Lock()
		defer mu.Unlock()
		json.NewEncoder(w).Encode(clients)
	case http.MethodPost:
		var client Client
		err := json.NewDecoder(r.Body).Decode(&client)
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		mu.Lock()
		clients[nextID] = client
		nextID++
		mu.Unlock()
		json.NewEncoder(w).Encode(client)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func main() {
	http.HandleFunc("/clients", clientHandler)
	log.Println("server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
