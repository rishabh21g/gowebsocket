package main

import (
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // allow all origins (dev)
	},
}

type Client struct {
	conn *websocket.Conn
	room string
}

var (
	rooms = make(map[string]map[*Client]bool)
	mu    sync.Mutex
)

func wsHandler(w http.ResponseWriter, r *http.Request) {
	// Strip leading slash from path to get room ID
	roomID := strings.TrimPrefix(r.URL.Path, "/")
	if roomID == "" {
		http.Error(w, "room required", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Upgrade error:", err)
		return
	}

	client := &Client{conn: conn, room: roomID}

	mu.Lock()
	if rooms[roomID] == nil {
		rooms[roomID] = make(map[*Client]bool)
	}
	rooms[roomID][client] = true
	mu.Unlock()

	log.Println("Client joined room:", roomID)

	defer func() {
		mu.Lock()
		delete(rooms[roomID], client)
		if len(rooms[roomID]) == 0 {
			delete(rooms, roomID)
		}
		mu.Unlock()
		conn.Close()
		log.Println("Client left room:", roomID)
	}()

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			log.Println("Read error:", err)
			break
		}

		log.Printf("Received message in room %s: %s", roomID, string(data))

		// Broadcast to all in same room (including sender)
		mu.Lock()
		for c := range rooms[roomID] {
			if err := c.conn.WriteMessage(msgType, data); err != nil {
				log.Println("Write error:", err)
			}
		}
		mu.Unlock()
	}
}

func main() {
	http.HandleFunc("/", wsHandler)

	log.Println("WS server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
