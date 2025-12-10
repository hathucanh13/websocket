package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"

	"github.com/gorilla/websocket"
)

type Message struct {
	Type     string `json:"type"`
	Room     string `json:"room"`
	Username string `json:"username"`
	Text     string `json:"text"`
	Time     string `json:"time"`
}

func main() {
	if len(os.Args) < 3 {
		os.Exit(1)
	}

	username := os.Args[1]
	room := os.Args[2]

	room = strings.TrimSpace(room)
	if room == "" {
		room = "general"
	}

	// Build WebSocket URL with query parameters
	u := url.URL{
		Scheme:   "ws",
		Host:     "localhost:8080",
		Path:     "/ws",
		RawQuery: fmt.Sprintf("username=%s&room=%s", username, room),
	}

	// Connect to WebSocket server
	conn, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer conn.Close()

	fmt.Printf("✓ Connected to room '%s' as '%s'\n", room, username)
	fmt.Println("Type messages and press Enter (Ctrl+C to exit)")
	fmt.Println("---")

	// Channel for interrupt signal
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Goroutine to read messages from server
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {

			_, data, err := conn.ReadMessage()
			if err != nil {
				log.Println("Connection closed:", err)
				return
			}

			var msg Message
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			// Display message based on type
			switch msg.Type {
			case "chat":
				fmt.Printf("[%s] %s: %s\n", msg.Time, msg.Username, msg.Text)
			case "system":
				fmt.Printf("[%s] * %s\n", msg.Time, msg.Text)
			case "user_list":
				fmt.Printf("[%s] * Users in room: %s\n", msg.Time, msg.Text)
			case "stats":
				fmt.Printf("[%s] * Global statistics: %s\n", msg.Time, msg.Text)
			case "room":
				fmt.Printf("[%s] * Available rooms: %s\n", msg.Time, msg.Text)
			default:
				// Unknown message type
			}
		}
	}()

	// Read input from user
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		// Send as JSON message
		msg := Message{
			Text: text,
		}
		data, _ := json.Marshal(msg)

		err := conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			log.Println("Write error:", err)
			return
		}
	}

	// Wait for goroutine to finish
	<-done
}
