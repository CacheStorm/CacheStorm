package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	cachestorm "github.com/cachestorm/cachestorm/clients/go"
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
	client, err := cachestorm.NewClient("localhost:6380")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Real-time Chat Example ===")
	fmt.Println("Commands: /join <room>, /leave, /users, /rooms, /quit")

	// Create default rooms
	createDefaultRooms(ctx, client)

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
	registerUser(ctx, client, user)

	currentRoom := "general"
	joinRoom(ctx, client, user, currentRoom)

	// Main loop
	fmt.Printf("\nJoined #%s. Start chatting!\n", currentRoom)
	for {
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
				leaveRoom(ctx, client, user, currentRoom)
				fmt.Println("Goodbye!")
				return

			case "/join":
				if len(parts) < 2 {
					fmt.Println("Usage: /join <room>")
					continue
				}
				leaveRoom(ctx, client, user, currentRoom)
				currentRoom = parts[1]
				joinRoom(ctx, client, user, currentRoom)

			case "/leave":
				leaveRoom(ctx, client, user, currentRoom)
				fmt.Println("Left room. Use /join <room> to enter another room.")

			case "/users":
				listUsers(ctx, client, currentRoom)

			case "/rooms":
				listRooms(ctx, client)

			default:
				fmt.Println("Unknown command:", cmd)
			}
		} else {
			// Send message
			sendMessage(ctx, client, user, currentRoom, input)
		}
	}
}

func createDefaultRooms(ctx context.Context, client *cachestorm.Client) {
	rooms := []Room{
		{ID: "general", Name: "General", Description: "General discussion", CreatedAt: time.Now().Unix()},
		{ID: "tech", Name: "Technology", Description: "Tech talk", CreatedAt: time.Now().Unix()},
		{ID: "random", Name: "Random", Description: "Random stuff", CreatedAt: time.Now().Unix()},
	}

	for _, room := range rooms {
		data, _ := json.Marshal(room)
		client.Set(ctx, fmt.Sprintf("room:%s", room.ID), string(data), 0)
	}
}

func registerUser(ctx context.Context, client *cachestorm.Client, user User) {
	data, _ := json.Marshal(user)
	client.Set(ctx, fmt.Sprintf("user:%s", user.ID), string(data), 0)
	client.SAdd(ctx, "users:online", user.ID)
}

func joinRoom(ctx context.Context, client *cachestorm.Client, user User, roomID string) {
	// Add user to room members
	client.SAdd(ctx, fmt.Sprintf("room:%s:members", roomID), user.ID)

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
	client.XAdd(ctx, fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))

	// Show recent messages
	fmt.Printf("\n--- Recent messages in #%s ---\n", roomID)
	messages, _ := client.XRevRange(ctx, fmt.Sprintf("room:%s:messages", roomID), "+", "-")
	limit := 10
	if len(messages) < limit {
		limit = len(messages)
	}
	for i := limit - 1; i >= 0; i-- {
		fmt.Printf("  message %d\n", i)
	}
	fmt.Println("---")
}

func leaveRoom(ctx context.Context, client *cachestorm.Client, user User, roomID string) {
	client.SRem(ctx, fmt.Sprintf("room:%s:members", roomID), user.ID)

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
	client.XAdd(ctx, fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))
}

func sendMessage(ctx context.Context, client *cachestorm.Client, user User, roomID, content string) {
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
	client.XAdd(ctx, fmt.Sprintf("room:%s:messages", roomID), "*", "data", string(msgJSON))

	// Update user's last activity
	client.Set(ctx, fmt.Sprintf("user:%s:last_seen", user.ID), fmt.Sprintf("%d", time.Now().Unix()), 0)
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

func listUsers(ctx context.Context, client *cachestorm.Client, roomID string) {
	members, _ := client.SMembers(ctx, fmt.Sprintf("room:%s:members", roomID))
	fmt.Printf("\nUsers in #%s:\n", roomID)
	for _, memberID := range members {
		userData, _ := client.Get(ctx, fmt.Sprintf("user:%s", memberID))
		var user User
		json.Unmarshal([]byte(userData), &user)
		fmt.Printf("  - %s (%s)\n", user.Username, user.Status)
	}
}

func listRooms(ctx context.Context, client *cachestorm.Client) {
	fmt.Println("\nAvailable rooms:")
	// List known rooms directly instead of using Keys (not available in client)
	roomIDs := []string{"general", "tech", "random"}
	for _, roomID := range roomIDs {
		roomData, _ := client.Get(ctx, fmt.Sprintf("room:%s", roomID))
		if roomData == "" {
			continue
		}
		var room Room
		json.Unmarshal([]byte(roomData), &room)
		memberCount, _ := client.SCard(ctx, fmt.Sprintf("room:%s:members", room.ID))
		fmt.Printf("  - #%s: %s (%d users)\n", room.ID, room.Name, memberCount)
	}
}
