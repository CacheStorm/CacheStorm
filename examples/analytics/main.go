package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cachestorm/cachestorm/clients/go"
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
	client, err := cachestorm.New("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("=== Real-time Analytics Example ===")

	// 1. Generate events
	fmt.Println("\n1. Generating analytics events...")
	generateEvents(client, 1000)

	// 2. Process events in real-time
	fmt.Println("\n2. Processing events...")
	processEvents(client)

	// 3. Calculate metrics
	fmt.Println("\n3. Calculating metrics...")
	calculateMetrics(client)

	// 4. Generate time-series data
	fmt.Println("\n4. Generating time-series data...")
	generateTimeSeries(client)

	// 5. Query analytics
	fmt.Println("\n5. Querying analytics...")
	queryAnalytics(client)

	// 6. Real-time dashboard simulation
	fmt.Println("\n6. Simulating real-time dashboard...")
	simulateDashboard(client)

	fmt.Println("\n=== Analytics Example Complete ===")
}

func generateEvents(client *cachestorm.Client, count int) {
	eventTypes := []string{"page_view", "click", "scroll", "purchase", "login", "logout"}
	pages := []string{"/home", "/products", "/cart", "/checkout", "/profile"}

	for i := 0; i < count; i++ {
		event := Event{
			EventID:   fmt.Sprintf("evt_%d_%d", time.Now().Unix(), i),
			EventType: eventTypes[rand.Intn(len(eventTypes))],
			UserID:    fmt.Sprintf("user_%d", rand.Intn(1000)),
			Timestamp: time.Now().Unix(),
			Data: map[string]interface{}{
				"page":      pages[rand.Intn(len(pages))],
				"duration":  rand.Intn(300),
				"device":    []string{"desktop", "mobile", "tablet"}[rand.Intn(3)],
				"referrer":  "https://google.com",
				"ip":        fmt.Sprintf("192.168.%d.%d", rand.Intn(256), rand.Intn(256)),
			},
		}

		eventJSON, _ := json.Marshal(event)

		// Add to time-series stream
		client.XAdd("events:stream", "*", "data", string(eventJSON))

		// Add to event type index
		client.SAdd(fmt.Sprintf("events:type:%s", event.EventType), event.EventID)

		// Add to user events
		client.LPush(fmt.Sprintf("user:%s:events", event.UserID), string(eventJSON))
		client.LTrim(fmt.Sprintf("user:%s:events", event.UserID), 0, 99)
	}

	fmt.Printf("  - Generated %d events\n", count)
}

func processEvents(client *cachestorm.Client) {
	// Read events from stream
	events, _ := client.XRange("events:stream", "-", "+")

	// Process each event
	for _, event := range events {
		var evt Event
		json.Unmarshal([]byte(event), &evt)

		// Update counters
		client.HIncrBy("stats:events:count", evt.EventType, 1)

		// Track unique users
		client.PFAdd("stats:unique_users", evt.UserID)

		// Track page views
		if page, ok := evt.Data["page"].(string); ok {
			client.ZIncrBy("stats:page_views", 1, page)
		}
	}

	// Get unique user count
	uniqueUsers, _ := client.PFCount("stats:unique_users")
	fmt.Printf("  - Unique users: %d\n", uniqueUsers)

	// Get event counts
	counts, _ := client.HGetAll("stats:events:count")
	fmt.Println("  - Event counts:")
	for eventType, count := range counts {
		fmt.Printf("    %s: %s\n", eventType, count)
	}
}

func calculateMetrics(client *cachestorm.Client) {
	// Calculate average session duration
	durations, _ := client.LRange("analytics:durations", 0, -1)
	var total int
	for _, d := range durations {
		var val int
		fmt.Sscanf(d, "%d", &val)
		total += val
	}
	if len(durations) > 0 {
		avg := float64(total) / float64(len(durations))
		client.Set("metrics:avg_session_duration", fmt.Sprintf("%.2f", avg))
		fmt.Printf("  - Average session duration: %.2f seconds\n", avg)
	}

	// Calculate conversion rate
	purchases, _ := client.SCard("events:type:purchase")
	pageViews, _ := client.SCard("events:type:page_view")
	if pageViews > 0 {
		conversionRate := float64(purchases) / float64(pageViews) * 100
		client.Set("metrics:conversion_rate", fmt.Sprintf("%.2f", conversionRate))
		fmt.Printf("  - Conversion rate: %.2f%%\n", conversionRate)
	}
}

func generateTimeSeries(client *cachestorm.Client) {
	// Generate CPU usage metrics for last 24 hours
	now := time.Now()
	for i := 0; i < 24; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour)
		score := float64(ts.Unix())
		value := 30.0 + rand.Float64()*40.0 // 30-70% CPU

		client.ZAdd("metrics:cpu_usage", value, fmt.Sprintf("%.0f", score))
	}

	// Generate memory usage
	for i := 0; i < 24; i++ {
		ts := now.Add(-time.Duration(i) * time.Hour)
		score := float64(ts.Unix())
		value := 40.0 + rand.Float64()*30.0 // 40-70% Memory

		client.ZAdd("metrics:memory_usage", value, fmt.Sprintf("%.0f", score))
	}

	// Generate request rate
	for i := 0; i < 60; i++ {
		ts := now.Add(-time.Duration(i) * time.Minute)
		score := float64(ts.Unix())
		value := 100.0 + rand.Float64()*500.0 // 100-600 req/min

		client.ZAdd("metrics:request_rate", value, fmt.Sprintf("%.0f", score))
	}

	fmt.Println("  - Generated 24h of CPU, memory, and request metrics")
}

func queryAnalytics(client *cachestorm.Client) {
	// Top pages
	fmt.Println("  - Top 5 pages:")
	topPages, _ := client.ZRevRangeWithScores("stats:page_views", 0, 4)
	for page, views := range topPages {
		fmt.Printf("    %s: %.0f views\n", page, views)
	}

	// CPU usage trend
	fmt.Println("  - CPU usage (last 5 hours):")
	cpuData, _ := client.ZRevRange("metrics:cpu_usage", 0, 4)
	for _, point := range cpuData {
		fmt.Printf("    %s%%\n", point)
	}

	// Recent events
	fmt.Println("  - Recent events:")
	recentEvents, _ := client.XRevRange("events:stream", "+", "-", 5)
	for _, event := range recentEvents {
		var evt Event
		json.Unmarshal([]byte(event), &evt)
		fmt.Printf("    %s: %s\n", evt.EventType, evt.UserID)
	}
}

func simulateDashboard(client *cachestorm.Client) {
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
			uniqueUsers, _ := client.PFCount("stats:unique_users")
			totalEvents, _ := client.XLen("events:stream")

			// Get last minute's request rate
			reqRate, _ := client.ZRevRange("metrics:request_rate", 0, 0)
			var rate string
			if len(reqRate) > 0 {
				rate = reqRate[0]
			}

			fmt.Printf("    Users: %d | Events: %d | Req/min: %s\n",
				uniqueUsers, totalEvents, rate)

		case <-done:
			fmt.Println("  - Dashboard simulation complete")
			return
		}
	}
}
