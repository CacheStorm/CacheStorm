package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	cachestorm "github.com/cachestorm/cachestorm/clients/go"
)

// Event represents an analytics event
type Event struct {
	EventID   string                 `json:"event_id"`
	EventType string                 `json:"event_type"`
	UserID    string                 `json:"user_id"`
	Timestamp int64                  `json:"timestamp"`
	Data      map[string]interface{} `json:"data"`
}

// Metric represents a time-series metric
type Metric struct {
	Name      string  `json:"name"`
	Value     float64 `json:"value"`
	Timestamp int64   `json:"timestamp"`
	Tags      map[string]string
}

func main() {
	client, err := cachestorm.NewClient("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	fmt.Println("=== Real-time Analytics Example ===")

	// 1. Generate events
	fmt.Println("\n1. Generating analytics events...")
	generateEvents(ctx, client, 1000)

	// 2. Process events in real-time
	fmt.Println("\n2. Processing events...")
	processEvents(ctx, client)

	// 3. Calculate metrics
	fmt.Println("\n3. Calculating metrics...")
	calculateMetrics(ctx, client)

	// 4. Generate time-series data
	fmt.Println("\n4. Generating time-series data...")
	generateTimeSeries(ctx, client)

	// 5. Query analytics
	fmt.Println("\n5. Querying analytics...")
	queryAnalytics(ctx, client)

	// 6. Real-time dashboard simulation
	fmt.Println("\n6. Simulating real-time dashboard...")
	simulateDashboard(ctx, client)

	fmt.Println("\n=== Analytics Example Complete ===")
}

func generateEvents(ctx context.Context, client *cachestorm.Client, count int) {
	eventTypes := []string{"page_view", "click", "scroll", "purchase", "login", "logout"}
	pages := []string{"/home", "/products", "/cart", "/checkout", "/profile"}

	for i := 0; i < count; i++ {
		event := Event{
			EventID:   fmt.Sprintf("evt_%d_%d", time.Now().Unix(), i),
			EventType: eventTypes[rand.Intn(len(eventTypes))],
			UserID:    fmt.Sprintf("user_%d", rand.Intn(1000)),
			Timestamp: time.Now().Unix(),
			Data: map[string]interface{}{
				"page":     pages[rand.Intn(len(pages))],
				"duration": rand.Intn(300),
				"device":   []string{"desktop", "mobile", "tablet"}[rand.Intn(3)],
				"referrer": "https://google.com",
				"ip":       fmt.Sprintf("192.168.%d.%d", rand.Intn(256), rand.Intn(256)),
			},
		}

		eventJSON, _ := json.Marshal(event)

		// Add to time-series stream
		client.XAdd(ctx, "events:stream", "*", "data", string(eventJSON))

		// Add to event type index
		client.SAdd(ctx, fmt.Sprintf("events:type:%s", event.EventType), event.EventID)

		// Add to user events
		client.LPush(ctx, fmt.Sprintf("user:%s:events", event.UserID), string(eventJSON))
		client.LTrim(ctx, fmt.Sprintf("user:%s:events", event.UserID), 0, 99)
	}

	fmt.Printf("  - Generated %d events\n", count)
}

func processEvents(ctx context.Context, client *cachestorm.Client) {
	// Read events from stream
	events, _ := client.XRange(ctx, "events:stream", "-", "+")

	// Process each event - XRange returns []interface{}, iterate over raw entries
	for range events {
		// Update counters
		client.HIncrBy(ctx, "stats:events:count", "processed", 1)
	}

	// Track unique users with sample data
	client.PFAdd(ctx, "stats:unique_users", "user_1", "user_2", "user_3")

	// Track page views
	client.ZIncrBy(ctx, "stats:page_views", 1, "/home")
	client.ZIncrBy(ctx, "stats:page_views", 1, "/products")

	// Get unique user count
	uniqueUsers, _ := client.PFCount(ctx, "stats:unique_users")
	fmt.Printf("  - Unique users: %d\n", uniqueUsers)

	// Get event counts
	counts, _ := client.HGetAll(ctx, "stats:events:count")
	fmt.Println("  - Event counts:")
	for eventType, count := range counts {
		fmt.Printf("    %s: %s\n", eventType, count)
	}
}

func calculateMetrics(ctx context.Context, client *cachestorm.Client) {
	// Calculate average session duration
	durations, _ := client.LRange(ctx, "analytics:durations", 0, -1)
	var total int
	for _, d := range durations {
		var val int
		fmt.Sscanf(d, "%d", &val)
		total += val
	}
	if len(durations) > 0 {
		avg := float64(total) / float64(len(durations))
		client.Set(ctx, "metrics:avg_session_duration", fmt.Sprintf("%.2f", avg), 0)
		fmt.Printf("  - Average session duration: %.2f seconds\n", avg)
	}

	// Calculate conversion rate
	purchases, _ := client.SCard(ctx, "events:type:purchase")
	pageViews, _ := client.SCard(ctx, "events:type:page_view")
	if pageViews > 0 {
		conversionRate := float64(purchases) / float64(pageViews) * 100
		client.Set(ctx, "metrics:conversion_rate", fmt.Sprintf("%.2f", conversionRate), 0)
		fmt.Printf("  - Conversion rate: %.2f%%\n", conversionRate)
	}
}

func generateTimeSeries(ctx context.Context, client *cachestorm.Client) {
	// Generate CPU usage metrics for last 24 hours
	now := time.Now()
	for i := 0; i < 24; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour)
		score := float64(ts.Unix())
		value := 30.0 + rand.Float64()*40.0 // 30-70% CPU

		client.ZAdd(ctx, "metrics:cpu_usage", value, fmt.Sprintf("%.0f", score))
	}

	// Generate memory usage
	for i := 0; i < 24; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour)
		score := float64(ts.Unix())
		value := 40.0 + rand.Float64()*30.0 // 40-70% Memory

		client.ZAdd(ctx, "metrics:memory_usage", value, fmt.Sprintf("%.0f", score))
	}

	// Generate request rate
	for i := 0; i < 60; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		score := float64(ts.Unix())
		value := 100.0 + rand.Float64()*500.0 // 100-600 req/min

		client.ZAdd(ctx, "metrics:request_rate", value, fmt.Sprintf("%.0f", score))
	}

	fmt.Println("  - Generated 24h of CPU, memory, and request metrics")
}

func queryAnalytics(ctx context.Context, client *cachestorm.Client) {
	// Top pages
	fmt.Println("  - Top 5 pages:")
	topPages, _ := client.ZRevRange(ctx, "stats:page_views", 0, 4)
	for _, page := range topPages {
		fmt.Printf("    %s\n", page)
	}

	// CPU usage trend
	fmt.Println("  - CPU usage (last 5 hours):")
	cpuData, _ := client.ZRevRange(ctx, "metrics:cpu_usage", 0, 4)
	for _, point := range cpuData {
		fmt.Printf("    %s%%\n", point)
	}

	// Recent events
	fmt.Println("  - Recent events:")
	recentEvents, _ := client.XRevRange(ctx, "events:stream", "+", "-")
	limit := 5
	if len(recentEvents) < limit {
		limit = len(recentEvents)
	}
	for i := 0; i < limit; i++ {
		fmt.Printf("    event %d\n", i)
	}
}

func simulateDashboard(ctx context.Context, client *cachestorm.Client) {
	// Simulate real-time dashboard updates
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	done := make(chan bool)
	go func() {
		time.Sleep(10 * time.Second)
		done <- true
	}()

	fmt.Println("  - Real-time metrics (10 seconds):")
	for {
		select {
		case <-ticker.C:
			// Get current metrics
			uniqueUsers, _ := client.PFCount(ctx, "stats:unique_users")

			// Get last minute's request rate
			reqRate, _ := client.ZRevRange(ctx, "metrics:request_rate", 0, 0)
			var rate string
			if len(reqRate) > 0 {
				rate = reqRate[0]
			}

			fmt.Printf("    Users: %d | Req/min: %s\n",
				uniqueUsers, rate)

		case <-done:
			fmt.Println("  - Dashboard simulation complete")
			return
		}
	}
}
