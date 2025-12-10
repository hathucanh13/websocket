package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const (
	MsgChat     = "chat"
	MsgSystem   = "system"
	MsgUserList = "user_list"
	MsgStats    = "stats"
	MsgCommand  = "command"
	MsgRoom     = "room"
)

type StatsMessage struct {
	TotalUsers  int            `json:"total_users"`
	TotalRooms  int            `json:"total_rooms"`
	RoomDetails map[string]int `json:"room_details"` // room -> user count
}

// Message types
type Message struct {
	Type     string `json:"type"` // "join", "leave", "chat", "system"
	Room     string `json:"room"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Time     string `json:"time"`
}

// Client represents a connected user
type Client struct {
	ID       string
	Username string
	Conn     *websocket.Conn
	Room     string
	Send     chan []byte
}

// Room represents a chat room
type Room struct {
	Name    string
	Clients map[*Client]bool
	mu      sync.RWMutex
}

// Hub manages all rooms and clients
type Hub struct {
	rooms      map[string]*Room
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

func newHub() *Hub {
	return &Hub{
		rooms:      make(map[string]*Room),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			log.Printf("Registering client: %s in room %s", client.Username, client.Room)
			h.addClientToRoom(client)

		case client := <-h.unregister:
			h.removeClientFromRoom(client)
		}
	}
}
func (h *Hub) handleCommand(client *Client, cmd string) {

	var msg Message
	userCount := make(map[string]int)
	room, exists := h.rooms[client.Room]
	if !exists {
		msg = Message{
			Type: MsgSystem,
			Text: "Room does not exist.",
		}
		// h.sendToClient(client, msg)
		data, _ := json.Marshal(msg)
		client.Send <- data
		return
	}
	// Send global statistics
	for r := range h.rooms {
		userCount[r] = len(h.rooms[r].Clients)
	}
	switch cmd {
	case "/users":
		var users []string
		for c := range room.Clients {
			users = append(users, c.Username)
		}
		msg = Message{
			Type:     MsgUserList,
			Room:     room.Name,
			Text:     strings.Join(users, ", "),
			Username: client.Username,
			Time:     time.Now().Format("15:04:05"),
		}
		h.sendToClient(client, msg)

	case "/stats":

		TotalUsers := 0
		for _, count := range userCount {
			TotalUsers += count
		}
		stats := StatsMessage{
			TotalUsers: TotalUsers,
			TotalRooms: len(h.rooms),
			// RoomDetails: userCount,
		}
		data, _ := json.Marshal(stats)
		msg = Message{
			Type:     MsgStats,
			Room:     room.Name,
			Text:     string(data),
			Username: client.Username,
			Time:     time.Now().Format("15:04:05"),
		}
		h.sendToClient(client, msg)
	case "/rooms":
		// Send list of all rooms
		data, _ := json.Marshal(userCount)
		msg = Message{
			Type:     MsgRoom,
			Room:     room.Name,
			Text:     string(data),
			Username: client.Username,
			Time:     time.Now().Format("15:04:05"),
		}
		h.sendToClient(client, msg)
	default:
		// Unknown command
		msg = Message{
			Type: MsgSystem,
			Text: "Unknown command. Available commands: /users, /stats, /rooms",
		}
		h.sendToClient(client, msg)
	}
}
func (h *Hub) addClientToRoom(client *Client) {
	h.mu.Lock()

	// Get or create room
	room, exists := h.rooms[client.Room]
	if !exists {
		log.Println("room does not exist, creating:", client.Room)
		room = &Room{
			Name:    client.Room,
			Clients: make(map[*Client]bool),
		}
		h.rooms[client.Room] = room
		log.Printf("Created new room: %s", client.Room)
	}
	log.Printf("Adding client %s to room %s", client.Username, client.Room)

	// Add client to room
	room.mu.Lock()
	room.Clients[client] = true
	room.mu.Unlock()

	log.Printf("Client %s joined room %s (Total: %d)",
		client.Username, client.Room, len(room.Clients))

	// Send join message to room
	msg := Message{
		Type: "system",
		Room: client.Room,
		Text: fmt.Sprintf("%s joined the room", client.Username),
		Time: time.Now().Format("15:04:05"),
	}
	h.mu.Unlock()
	h.broadcastToRoom(client.Room, msg)
}

func (h *Hub) removeClientFromRoom(client *Client) {
	h.mu.RLock()
	room, exists := h.rooms[client.Room]
	h.mu.RUnlock()

	if !exists {
		return
	}

	room.mu.Lock()
	if _, ok := room.Clients[client]; ok {
		delete(room.Clients, client)
		close(client.Send)
	}
	room.mu.Unlock()

	log.Printf("Client %s left room %s (Remaining: %d)",
		client.Username, client.Room, len(room.Clients))

	// Send leave message to room
	msg := Message{
		Type: "system",
		Room: client.Room,
		Text: fmt.Sprintf("%s left the room", client.Username),
		Time: time.Now().Format("15:04:05"),
	}
	h.broadcastToRoom(client.Room, msg)

	// Delete room if empty
	if len(room.Clients) == 0 {
		h.mu.Lock()
		delete(h.rooms, client.Room)
		h.mu.Unlock()
		log.Printf("Deleted empty room: %s", client.Room)
	}
}

func (h *Hub) broadcastToRoom(roomName string, msg Message) {
	h.mu.RLock()
	room, exists := h.rooms[roomName]
	h.mu.RUnlock()

	if !exists {
		return
	}

	data, _ := json.Marshal(msg)

	room.mu.RLock()
	defer room.mu.RUnlock()

	for client := range room.Clients {
		select {
		case client.Send <- data:
		default:
			close(client.Send)
			delete(room.Clients, client)
		}
	}
}

func (h *Hub) sendToClient(client *Client, msg Message) {
	data, _ := json.Marshal(msg)
	log.Printf("Sending message to client %s: %s", client.Username, string(data))
	select {
	case client.Send <- data:
		log.Printf("Message sent to channel %s", client.Username)
	default:
		close(client.Send)

	}
}

func (c *Client) readPump(hub *Hub) {
	defer func() {
		hub.unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		log.Println("Received message:", string(data))
		if err != nil {
			break
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		if strings.HasPrefix(msg.Text, "/") {
			log.Println("Received command:", msg.Text)
			hub.handleCommand(c, msg.Text)
			continue
		}

		// Set message metadata
		msg.Username = c.Username
		msg.Room = c.Room
		msg.Type = "chat"
		msg.Time = time.Now().Format("15:04:05")

		// Broadcast to room
		hub.broadcastToRoom(c.Room, msg)
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(54 * time.Second)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if !ok {
				log.Println("Client send channel closed")
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				log.Println("Write error:", err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

var hub = newHub()

func handleWebSocket(c *gin.Context) {

	username := c.Query("username")
	room := c.Query("room")
	log.Printf("Connection request: username=%s, room=%s", username, room)

	if username == "" || room == "" {
		c.JSON(400, gin.H{"error": "username and room required"})
		return
	}
	room = strings.TrimSpace(room)

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Upgrade failed: %v", err)
		return
	}

	client := &Client{
		ID:       fmt.Sprintf("%s-%d", username, time.Now().Unix()),
		Username: username,
		Room:     room,
		Conn:     conn,
		Send:     make(chan []byte, 256),
	}
	log.Printf("New client created: %s in room %s", client.Username, client.Room)

	hub.register <- client

	go client.writePump()
	go client.readPump(hub)

}

func main() {
	go hub.run()

	router := gin.Default()
	router.GET("/ws", handleWebSocket)

	// Serve static files (HTML, JS, CSS)
	router.Static("/static", "./static")
	router.LoadHTMLFiles("static/index.html")

	// Serve the UI at "/"
	router.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", nil)
	})

	fmt.Println("🚀 Chat Rooms Server started on :8080")
	fmt.Println("📱 Connect using: go run client/room_client.go <username> <room>")

	router.Run(":8080")
}
