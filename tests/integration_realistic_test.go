package tests

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/server"
)

// RealisticDataGenerator generates realistic test data
type RealisticDataGenerator struct {
	rng *rand.Rand
}

func NewRealisticDataGenerator() *RealisticDataGenerator {
	return &RealisticDataGenerator{
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// GenerateUserSession generates realistic user session data
func (g *RealisticDataGenerator) GenerateUserSession(userID int) map[string]interface{} {
	sessions := []string{"web", "mobile", "api", "desktop"}
	countries := []string{"US", "UK", "DE", "FR", "TR", "JP", "BR", "IN"}
	browsers := []string{"Chrome", "Firefox", "Safari", "Edge"}

	return map[string]interface{}{
		"user_id":     userID,
		"session_id":  fmt.Sprintf("sess_%d_%d", userID, g.rng.Int63()),
		"platform":    sessions[g.rng.Intn(len(sessions))],
		"country":     countries[g.rng.Intn(len(countries))],
		"browser":     browsers[g.rng.Intn(len(browsers))],
		"login_time":  time.Now().Add(-time.Duration(g.rng.Intn(3600)) * time.Second).Unix(),
		"last_active": time.Now().Unix(),
		"ip":          fmt.Sprintf("192.168.%d.%d", g.rng.Intn(256), g.rng.Intn(256)),
	}
}

// GenerateProduct generates realistic product data
func (g *RealisticDataGenerator) GenerateProduct(productID int) map[string]interface{} {
	categories := []string{"electronics", "clothing", "food", "books", "home", "sports"}
	brands := []string{"TechCorp", "StyleInc", "FreshFoods", "BookHaven", "HomeComfort", "SportsPro"}

	return map[string]interface{}{
		"id":          productID,
		"name":        fmt.Sprintf("Product_%d_%s", productID, g.randomString(8)),
		"category":    categories[g.rng.Intn(len(categories))],
		"brand":       brands[g.rng.Intn(len(brands))],
		"price":       float64(g.rng.Intn(10000)) / 100.0,
		"stock":       g.rng.Intn(1000),
		"rating":      float64(g.rng.Intn(50)) / 10.0,
		"reviews":     g.rng.Intn(10000),
		"created_at":  time.Now().Add(-time.Duration(g.rng.Intn(365*24)) * time.Hour).Format(time.RFC3339),
		"tags":        g.randomTags(g.rng.Intn(5) + 1),
		"description": g.randomString(100),
	}
}

// GenerateOrder generates realistic order data
func (g *RealisticDataGenerator) GenerateOrder(orderID int) map[string]interface{} {
	statuses := []string{"pending", "processing", "shipped", "delivered", "cancelled"}
	paymentMethods := []string{"credit_card", "paypal", "bank_transfer", "crypto"}

	items := make([]map[string]interface{}, g.rng.Intn(5)+1)
	for i := range items {
		items[i] = map[string]interface{}{
			"product_id": g.rng.Intn(10000),
			"quantity":   g.rng.Intn(10) + 1,
			"price":      float64(g.rng.Intn(10000)) / 100.0,
		}
	}

	return map[string]interface{}{
		"order_id":       orderID,
		"user_id":        g.rng.Intn(100000),
		"status":         statuses[g.rng.Intn(len(statuses))],
		"payment_method": paymentMethods[g.rng.Intn(len(paymentMethods))],
		"items":          items,
		"total":          float64(g.rng.Intn(50000)) / 100.0,
		"shipping":       float64(g.rng.Intn(2000)) / 100.0,
		"tax":            float64(g.rng.Intn(5000)) / 100.0,
		"created_at":     time.Now().Add(-time.Duration(g.rng.Intn(30*24)) * time.Hour).Format(time.RFC3339),
		"address": map[string]string{
			"street":  fmt.Sprintf("%d %s St", g.rng.Intn(9999), g.randomString(10)),
			"city":    g.randomString(8),
			"country": "US",
			"zip":     fmt.Sprintf("%05d", g.rng.Intn(99999)),
		},
	}
}

// GenerateAnalyticsEvent generates realistic analytics event
func (g *RealisticDataGenerator) GenerateAnalyticsEvent(eventID int) map[string]interface{} {
	eventTypes := []string{"page_view", "click", "scroll", "purchase", "login", "logout", "search", "add_to_cart"}
	pages := []string{"/home", "/products", "/cart", "/checkout", "/profile", "/search", "/about"}

	return map[string]interface{}{
		"event_id":   eventID,
		"event_type": eventTypes[g.rng.Intn(len(eventTypes))],
		"user_id":    g.rng.Intn(100000),
		"session_id": fmt.Sprintf("sess_%d", g.rng.Int63()),
		"page":       pages[g.rng.Intn(len(pages))],
		"timestamp":  time.Now().Unix(),
		"metadata": map[string]interface{}{
			"referrer": g.randomString(20),
			"device":   []string{"desktop", "mobile", "tablet"}[g.rng.Intn(3)],
			"duration": g.rng.Intn(300),
		},
	}
}

func (g *RealisticDataGenerator) randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[g.rng.Intn(len(charset))]
	}
	return string(b)
}

func (g *RealisticDataGenerator) randomTags(count int) []string {
	tags := []string{"new", "sale", "popular", "limited", "exclusive", "trending", "bestseller", "featured"}
	result := make([]string, count)
	for i := 0; i < count; i++ {
		result[i] = tags[g.rng.Intn(len(tags))]
	}
	return result
}

// TestServer represents a running test server
type TestServer struct {
	addr   string
	server *server.Server
	wg     sync.WaitGroup
}

func StartTestServer(t *testing.T) *TestServer {
	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	addr := fmt.Sprintf("127.0.0.1:%d", port)

	// Create server config
	cfg := &config.Config{
		Server: config.ServerConfig{
			Bind:            "127.0.0.1",
			Port:            port,
			MaxConnections:  10000,
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
		},
		HTTP: config.HTTPConfig{
			Enabled: false,
		},
	}

	// Create server
	srv, err := server.New(cfg)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	ts := &TestServer{
		addr:   addr,
		server: srv,
	}

	// Start server in background
	ts.wg.Add(1)
	go func() {
		defer ts.wg.Done()
		srv.Start(context.Background())
	}()

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Verify server is running
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("Server failed to start: %v", err)
	}
	conn.Close()

	t.Logf("Test server started on %s", addr)
	return ts
}

func (ts *TestServer) Stop() {
	ts.server.Stop(context.Background())
	ts.wg.Wait()
}

func (ts *TestServer) Addr() string {
	return ts.addr
}

// RedisClient simple RESP client for testing
type RedisClient struct {
	conn   net.Conn
	reader *bufio.Reader
	addr   string
}

func NewRedisClient(addr string) (*RedisClient, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &RedisClient{
		conn:   conn,
		reader: bufio.NewReader(conn),
		addr:   addr,
	}, nil
}

func (c *RedisClient) Close() error {
	return c.conn.Close()
}

func (c *RedisClient) Send(args ...interface{}) (interface{}, error) {
	// Build RESP command
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, arg := range args {
		switch v := arg.(type) {
		case string:
			cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
		case int:
			s := fmt.Sprintf("%d", v)
			cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
		case []byte:
			cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(v), v)
		default:
			s := fmt.Sprintf("%v", v)
			cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(s), s)
		}
	}

	if _, err := c.conn.Write([]byte(cmd)); err != nil {
		return nil, err
	}

	return c.readResponse()
}

func (c *RedisClient) readResponse() (interface{}, error) {
	line, err := c.reader.ReadString('\n')
	if err != nil {
		return nil, err
	}

	if len(line) < 1 {
		return nil, fmt.Errorf("empty response")
	}

	switch line[0] {
	case '+': // Simple string
		return strings.TrimRight(line[1:], "\r\n"), nil
	case '-': // Error
		return nil, fmt.Errorf("RESP error: %s", strings.TrimRight(line[1:], "\r\n"))
	case ':': // Integer
		var n int64
		fmt.Sscanf(line[1:], "%d", &n)
		return n, nil
	case '$': // Bulk string
		var length int
		fmt.Sscanf(line[1:], "%d", &length)
		if length == -1 {
			return nil, nil // NULL
		}
		data := make([]byte, length+2) // +2 for \r\n
		if _, err := c.reader.Read(data); err != nil {
			return nil, err
		}
		return string(data[:length]), nil
	case '*': // Array
		var count int
		fmt.Sscanf(line[1:], "%d", &count)
		if count == -1 {
			return nil, nil // NULL array
		}
		arr := make([]interface{}, count)
		for i := 0; i < count; i++ {
			val, err := c.readResponse()
			if err != nil {
				return nil, err
			}
			arr[i] = val
		}
		return arr, nil
	default:
		return nil, fmt.Errorf("unknown RESP type: %c", line[0])
	}
}

// BenchmarkRealisticWorkload tests realistic workload patterns
func BenchmarkRealisticWorkload(b *testing.B) {
	ts := StartTestServer(&testing.T{})
	defer ts.Stop()

	client, err := NewRedisClient(ts.Addr())
	if err != nil {
		b.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	gen := NewRealisticDataGenerator()

	// Pre-populate with realistic data
	b.Log("Pre-populating data...")
	for i := 0; i < 10000; i++ {
		// User sessions
		session := gen.GenerateUserSession(i)
		sessionJSON, _ := json.Marshal(session)
		client.Send("SET", fmt.Sprintf("session:%d", i), string(sessionJSON), "EX", "3600")

		// Products
		product := gen.GenerateProduct(i)
		productJSON, _ := json.Marshal(product)
		client.Send("SET", fmt.Sprintf("product:%d", i), string(productJSON))

		// Add to various indices
		client.Send("SADD", fmt.Sprintf("category:%s", product["category"]), fmt.Sprintf("%d", i))
		client.Send("ZADD", "products:by_rating", fmt.Sprintf("%v", product["rating"]), fmt.Sprintf("%d", i))
		client.Send("ZADD", "products:by_price", fmt.Sprintf("%v", product["price"]), fmt.Sprintf("%d", i))
	}

	b.Log("Starting benchmark...")
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		client, _ := NewRedisClient(ts.Addr())
		defer client.Close()

		// Each goroutine gets its own generator to avoid race on shared rand.Rand
		localGen := NewRealisticDataGenerator()

		opCount := 0
		for pb.Next() {
			op := opCount % 100
			opCount++

			switch {
			case op < 40: // 40% reads
				// Read session
				userID := rand.Intn(10000)
				client.Send("GET", fmt.Sprintf("session:%d", userID))

			case op < 60: // 20% product reads
				productID := rand.Intn(10000)
				client.Send("GET", fmt.Sprintf("product:%d", productID))

			case op < 75: // 15% set operations
				category := []string{"electronics", "clothing", "food", "books", "home", "sports"}[rand.Intn(6)]
				client.Send("SMEMBERS", fmt.Sprintf("category:%s", category))

			case op < 85: // 10% sorted set queries
				client.Send("ZRANGEBYSCORE", "products:by_rating", "4.0", "5.0", "LIMIT", "0", "10")

			case op < 95: // 10% writes
				session := localGen.GenerateUserSession(rand.Intn(100000))
				sessionJSON, _ := json.Marshal(session)
				client.Send("SET", fmt.Sprintf("session:%d", session["user_id"]), string(sessionJSON), "EX", "3600")

			default: // 5% analytics
				event := localGen.GenerateAnalyticsEvent(rand.Intn(1000000))
				eventJSON, _ := json.Marshal(event)
				client.Send("LPUSH", "analytics:events", string(eventJSON))
				client.Send("LTRIM", "analytics:events", "0", "9999")
			}
		}
	})
}

// BenchmarkHighConcurrency tests high concurrency scenarios
func BenchmarkHighConcurrency(b *testing.B) {
	ts := StartTestServer(&testing.T{})
	defer ts.Stop()

	// Pre-populate
	client, _ := NewRedisClient(ts.Addr())
	for i := 0; i < 1000; i++ {
		client.Send("SET", fmt.Sprintf("key:%d", i), fmt.Sprintf("value:%d", i))
	}
	client.Close()

	concurrencyLevels := []int{10, 50, 100, 500}

	for _, concurrency := range concurrencyLevels {
		b.Run(fmt.Sprintf("concurrency_%d", concurrency), func(b *testing.B) {
			b.SetParallelism(concurrency)
			b.RunParallel(func(pb *testing.PB) {
				client, err := NewRedisClient(ts.Addr())
				if err != nil {
					b.Logf("Failed to connect: %v", err)
					return
				}
				defer client.Close()

				for pb.Next() {
					key := fmt.Sprintf("key:%d", rand.Intn(1000))
					if rand.Float32() < 0.8 {
						client.Send("GET", key)
					} else {
						client.Send("SET", key, fmt.Sprintf("value:%d", rand.Intn(1000000)))
					}
				}
			})
		})
	}
}

// BenchmarkDataStructures tests various data structures
func BenchmarkDataStructures(b *testing.B) {
	ts := StartTestServer(&testing.T{})
	defer ts.Stop()

	client, _ := NewRedisClient(ts.Addr())
	defer client.Close()

	b.Run("Strings", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("SET", fmt.Sprintf("str:%d", i), fmt.Sprintf("value:%d", i))
		}
	})

	b.Run("Hashes", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("HSET", fmt.Sprintf("hash:%d", i), "field1", "value1", "field2", "value2")
		}
	})

	b.Run("Lists", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("LPUSH", fmt.Sprintf("list:%d", i%100), fmt.Sprintf("item:%d", i))
		}
	})

	b.Run("Sets", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("SADD", fmt.Sprintf("set:%d", i%100), fmt.Sprintf("member:%d", i))
		}
	})

	b.Run("SortedSets", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			client.Send("ZADD", fmt.Sprintf("zset:%d", i%100), fmt.Sprintf("%d", i), fmt.Sprintf("member:%d", i))
		}
	})
}

// TestE2ERealisticScenario tests end-to-end realistic scenarios
func TestE2ERealisticScenario(t *testing.T) {
	ts := StartTestServer(t)
	defer ts.Stop()

	client, err := NewRedisClient(ts.Addr())
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	gen := NewRealisticDataGenerator()

	t.Run("E-commerce Flow", func(t *testing.T) {
		// 1. Store products
		for i := 0; i < 100; i++ {
			product := gen.GenerateProduct(i)
			productJSON, _ := json.Marshal(product)
			_, err := client.Send("SET", fmt.Sprintf("product:%d", i), string(productJSON))
			if err != nil {
				t.Fatalf("Failed to store product: %v", err)
			}
		}

		// 2. Create shopping cart
		userID := 12345
		cartKey := fmt.Sprintf("cart:%d", userID)

		// Add items to cart
		for i := 0; i < 3; i++ {
			productID := rand.Intn(100)
			_, err := client.Send("HSET", cartKey,
				fmt.Sprintf("product:%d", productID),
				fmt.Sprintf(`{"qty":%d,"price":%.2f}`, rand.Intn(5)+1, float64(rand.Intn(10000))/100.0))
			if err != nil {
				t.Fatalf("Failed to add to cart: %v", err)
			}
		}

		// 3. Set cart expiry
		_, err = client.Send("EXPIRE", cartKey, "3600")
		if err != nil {
			t.Fatalf("Failed to set expiry: %v", err)
		}

		// 4. Get cart contents
		cart, err := client.Send("HGETALL", cartKey)
		if err != nil {
			t.Fatalf("Failed to get cart: %v", err)
		}
		if cart == nil {
			t.Fatal("Cart is empty")
		}

		t.Logf("Cart has items")
	})

	t.Run("Session Management", func(t *testing.T) {
		// Create sessions for multiple users
		for i := 0; i < 10; i++ {
			session := gen.GenerateUserSession(i)
			sessionJSON, _ := json.Marshal(session)

			// Store session with TTL
			_, err := client.Send("SET", fmt.Sprintf("session:%d", i), string(sessionJSON), "EX", "3600")
			if err != nil {
				t.Fatalf("Failed to store session: %v", err)
			}

			// Add to active sessions set
			_, err = client.Send("SADD", "sessions:active", fmt.Sprintf("%d", i))
			if err != nil {
				t.Fatalf("Failed to add to active sessions: %v", err)
			}
		}

		// Get active session count
		count, err := client.Send("SCARD", "sessions:active")
		if err != nil {
			t.Fatalf("Failed to get session count: %v", err)
		}
		t.Logf("Active sessions: %v", count)
	})

	t.Run("Real-time Analytics", func(t *testing.T) {
		// Simulate analytics events
		for i := 0; i < 100; i++ {
			event := gen.GenerateAnalyticsEvent(i)
			eventJSON, _ := json.Marshal(event)

			// Add to time-series stream
			_, err := client.Send("XADD", "analytics:stream", "*", "event", string(eventJSON))
			if err != nil {
				t.Fatalf("Failed to add event: %v", err)
			}
		}

		// Get stream length
		len, err := client.Send("XLEN", "analytics:stream")
		if err != nil {
			t.Fatalf("Failed to get stream length: %v", err)
		}
		t.Logf("Analytics stream length: %v", len)
	})
}
