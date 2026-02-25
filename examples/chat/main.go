package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/cachestorm/cachestorm/clients/go/cachestorm"
)

// Message represents a chat message
type Message struct {
	ID        string `json:"id"`
	RoomID    string `json:"room_id"`
	UserID    string `json:"user_id"`
	Username  string `json:"username"`
	Content   string `json:"content"`
	Timestamp int64  `json:"timestamp"`
	Type      string `json:"type"` // text, image, system
}

// User represents a chat user
type User struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Status   string `json:"status"` // online, away, offline
	LastSeen int64  `json:"last_seen"`
}

// Room represents a chat room
type Room struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Members     []string `json:"members"`
	CreatedAt   int64    `json:"created_at"`
}

func main() {
	client, err := cachestorm.New("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("=== Real-time Chat Example ===")
	fmt.Println("Commands: /join <room>, /leave, /users, /rooms, /quit")

	// Create default rooms
	createDefaultRooms(client)

	// Get user info
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter your username: ")
	username, _ := reader.ReadString('\n')
	username = strings.TrimSpace(username)

	userID := fmt.Sprintf("user_%d", time.Now().Unix())
	user := User{
		ID:       userID,
		Username: username,
		Status:   "online",
		LastSeen: time.Now().Unix(),
	}

	// Register user
	registerUser(client, user)

	currentRoom := "general"
	joinRoom(client, user, currentRoom)

	// Message listener
	msgChan := make(chan Message)
	go listenForMessages(client, currentRoom, msgChan)

	// Main loop
	fmt.Printf("\nJoined #%s. Start chatting!\n", currentRoom)
	for {
		select {
		case msg := <-msgChan:
			if msg.RoomID == currentRoom {
				displayMessage(msg)
			}
		default:
			fmt.Print("> ")
			input, _ := reader.ReadString('\n')
			input = strings.TrimSpace(input)

			if input == "" {
				continue
			}

			// Handle commands
			if strings.HasPrefix(input, "/") {
				parts := strings.Fields(input)
				cmd := parts[0]

				switch cmd {
				case "/quit":
					leaveRoom(client, user, currentRoom)
					fmt.Println("Goodbye!")
					return

				case "/join":
					if len(parts) < 2 {
						fmt.Println("Usage: /join <room>")
						continue
					}
					leaveRoom(client, user, currentRoom)
					currentRoom = parts[1]
					joinRoom(client, user, currentRoom)

				case "/leave":
					leaveRoom(client, user, currentRoom)
					fmt.Println("Left room. Use /join <room> to enter another room.")

				case "/users":
					listUsers(client, currentRoom)

				case "/rooms":
					listRooms(client)

				default:
					fmt.Println("Unknown command:", cmd)
				}
			} else {
				// Send message
				sendMessage(client, user, currentRoom, input)
			}
		}
	}
}

func createDefaultRooms(client *cachestorm.Client) {
	rooms := []Room{
		{ID: "general", Name: "General", Description: "General discussion", CreatedAt: time.Now().Unix()},
		{ID: "tech", Name: "Technology", Description: "Tech talk", CreatedAt: time.Now().Unix()},
		{ID: "random", Name: "Random", Description: "Random stuff", CreatedAt: time.Now().Unix()},
	}

	for _, room := range rooms {
		data, _ := json.Marshal(room)
		client.Set(fmt.Sprintf("room:%s", room.ID), string(data))
	}
}

func registerUser(client *cachestorm.Client, user User) {
	data, _ := json.Marshal(user)
	client.Set(fmt.Sprintf("user:%s", user.ID), string(data))
	client.SAdd("users:online", user.ID)
}

func joinRoom(client *cachestorm.Client, user User, roomID string) {
	// Add user to room members
	client.SAdd(fmt.Sprintf("room:%s:members", roomID), user.ID)

	// Send join message
	msg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		RoomID:    roomID,
		UserID:    "system",
		Username:  "System",
		Content:   fmt.Sprintf("%s joined the room", user.Username),
		Timestamp: time.Now().Unix(),
		Type:      "system",
	}

	msgJSON, _ := json.Marshal(msg)
	client.XAdd(fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))

	// Show recent messages
	fmt.Printf("\n--- Recent messages in #%s ---\n", roomID)
	messages, _ := client.XRevRange(fmt.Sprintf("room:%s:messages", roomID), "+", "-", 10)
	for i := len(messages) - 1; i >= 0; i-- {
		var m Message
		json.Unmarshal([]byte(messages[i]), &m)
		displayMessage(m)
	}
	fmt.Println("---")
}

func leaveRoom(client *cachestorm.Client, user User, roomID string) {
	client.SRem(fmt.Sprintf("room:%s:members", roomID), user.ID)

	msg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		RoomID:    roomID,
		UserID:    "system",
		Username:  "System",
		Content:   fmt.Sprintf("%s left the room", user.Username),
		Timestamp: time.Now().Unix(),
		Type:      "system",
	}

	msgJSON, _ := json.Marshal(msg)
	client.XAdd(fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))
}

func sendMessage(client *cachestorm.Client, user User, roomID, content string) {
	msg := Message{
		ID:        fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		RoomID:    roomID,
		UserID:    user.ID,
		Username:  user.Username,
		Content:   content,
		Timestamp: time.Now().Unix(),
		Type:      "text",
	}

	msgJSON, _ := json.Marshal(msg)
	client.XAdd(fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))

	// Update user's last activity
	client.Set(fmt.Sprintf("user:%s:last_seen", user.ID), fmt.Sprintf("%d", time.Now().Unix()))
}

func listenForMessages(client *cachestorm.Client, roomID string, msgChan chan<- Message) {
	lastID := "0"

	for {
		// Read new messages from stream
		messages, _ := client.XRead(fmt.Sprintf("room:%s:messages", roomID), lastID, 1, 5000)

		for _, data := range messages {
			var msg Message
			json.Unmarshal([]byte(data), &msg)
			msgChan <- msg
			lastID = msg.ID
		}
	}
}

func displayMessage(msg Message) {
	timeStr := time.Unix(msg.Timestamp, 0).Format("15:04:05")

	switch msg.Type {
	case "system":
		fmt.Printf("[%s] *** %s ***\n", timeStr, msg.Content)
	default:
		fmt.Printf("[%s] %s: %s\n", timeStr, msg.Username, msg.Content)
	}
}

func listUsers(client *cachestorm.Client, roomID string) {
	members, _ := client.SMembers(fmt.Sprintf("room:%s:members", roomID))
	fmt.Printf("\nUsers in #%s:\n", roomID)
	for _, memberID := range members {
		userData, _ := client.Get(fmt.Sprintf("user:%s", memberID))
		var user User
		json.Unmarshal([]byte(userData), &user)
		fmt.Printf("  - %s (%s)\n", user.Username, user.Status)
	}
}

func listRooms(client *cachestorm.Client) {
	fmt.Println("\nAvailable rooms:")
	roomKeys, _ := client.Keys("room:*")
	for _, key := range roomKeys {
		if !strings.Contains(key, ":members") && !strings.Contains(key, ":messages") {
			roomData, _ := client.Get(key)
			var room Room
			json.Unmarshal([]byte(roomData), &room)
			memberCount, _ := client.SCard(fmt.Sprintf("room:%s:members", room.ID))
			fmt.Printf("  - #%s: %s (%d users)\n", room.ID, room.Name, memberCount)
		}
	}
}
