package main

import (
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"golang.org/x/net/context"
)

// Set up global variables
var (
	clients  = make(map[*websocket.Conn]bool) // Active clients
	mutex    = &sync.Mutex{}                  // Mutex to protect the clients map
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	ctx = context.Background()
)

// Establish WebSocket connection
func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	// Add the new WebSocket connection to the global clients map
	mutex.Lock()
	clients[conn] = true
	mutex.Unlock()

	// Wait for the connection to close (or expand to handle incoming messages from the client)
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			break
		}
	}

	// Remove the WebSocket connection from the global clients map once it's closed
	mutex.Lock()
	delete(clients, conn)
	mutex.Unlock()
}

func listenToRedisAndBroadcast() {
	rdb := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_URL"), // Change to your Redis address
	})

	pubsub := rdb.Subscribe(ctx, "notifications")
	ch := pubsub.Channel()

	for msg := range ch {
		// Broadcast the message to all connected WebSocket clients
		mutex.Lock()
		for client := range clients {
			if err := client.WriteMessage(websocket.TextMessage, []byte(msg.Payload)); err != nil {
				log.Println(err)
				client.Close()
				delete(clients, client)
			}
		}
		mutex.Unlock()
	}
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	// WebSocket setup
	http.HandleFunc("/ws", handleConnection)
	go listenToRedisAndBroadcast()
	log.Println("WebSocket server started on :8080...")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
