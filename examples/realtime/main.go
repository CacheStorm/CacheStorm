package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cachestorm/cachestorm/clients/go"
)

// SensorData represents IoT sensor data
type SensorData struct {
	SensorID    string                 `json:"sensor_id"`
	Type        string                 `json:"type"`
	Location    string                 `json:"location"`
	Value       float64                `json:"value"`
	Unit        string                 `json:"unit"`
	Timestamp   int64                  `json:"timestamp"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// Alert represents a system alert
type Alert struct {
	ID          string    `json:"id"`
	Level       string    `json:"level"`
	Message     string    `json:"message"`
	Source      string    `json:"source"`
	Timestamp   int64     `json:"timestamp"`
	Acknowledged bool     `json:"acknowledged"`
}

func main() {
	client, err := cachestorm.New("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("=== Real-time IoT Monitoring Example ===")

	// 1. Initialize sensors
	fmt.Println("\n1. Initializing sensors...")
	initializeSensors(client)

	// 2. Start sensor data simulation
	fmt.Println("\n2. Starting sensor data simulation...")
	go simulateSensors(client)

	// 3. Start alerting system
	fmt.Println("\n3. Starting alerting system...")
	go monitorAlerts(client)

	// 4. Start dashboard
	fmt.Println("\n4. Starting real-time dashboard...")
	runDashboard(client)
}

func initializeSensors(client *cachestorm.Client) {
	sensors := []map[string]interface{}{
		{"id": "temp_001", "type": "temperature", "location": "Server Room A", "unit": "°C", "min": 18, "max": 25},
		{"id": "temp_002", "type": "temperature", "location": "Server Room B", "unit": "°C", "min": 18, "max": 25},
		{"id": "humidity_001", "type": "humidity", "location": "Data Center", "unit": "%", "min": 40, "max": 60},
		{"id": "cpu_001", "type": "cpu", "location": "Server-01", "unit": "%", "min": 0, "max": 80},
		{"id": "cpu_002", "type": "cpu", "location": "Server-02", "unit": "%", "min": 0, "max": 80},
		{"id": "memory_001", "type": "memory", "location": "Server-01", "unit": "%", "min": 0, "max": 90},
		{"id": "network_001", "type": "bandwidth", "location": "Core Switch", "unit": "Mbps", "min": 0, "max": 10000},
	}

	for _, sensor := range sensors {
		data, _ := json.Marshal(sensor)
		client.Set(fmt.Sprintf("sensor:%s", sensor["id"]), string(data))
		client.SAdd("sensors:active", sensor["id"])
	}

	fmt.Printf("  - Initialized %d sensors\n", len(sensors))
}

func simulateSensors(client *cachestorm.Client) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		sensors, _ := client.SMembers("sensors:active")

		for _, sensorID := range sensors {
			sensorData, _ := client.Get(fmt.Sprintf("sensor:%s", sensorID))
			var sensor map[string]interface{}
			json.Unmarshal([]byte(sensorData), &sensor)

			sensorType := sensor["type"].(string)
			minVal := sensor["min"].(float64)
			maxVal := sensor["max"].(float64)

			// Generate realistic value
			var value float64
			switch sensorType {
			case "temperature":
				value = minVal + rand.Float64()*(maxVal-minVal)
			case "humidity":
				value = minVal + rand.Float64()*(maxVal-minVal)
			case "cpu":
				value = rand.Float64() * 100
			case "memory":
				value = 30 + rand.Float64()*40
			case "bandwidth":
				value = rand.Float64() * 8000
			}

			data := SensorData{
				SensorID:  sensorID,
				Type:      sensorType,
				Location:  sensor["location"].(string),
				Value:     value,
				Unit:      sensor["unit"].(string),
				Timestamp: time.Now().Unix(),
				Metadata: map[string]interface{}{
					"threshold_min": minVal,
					"threshold_max": maxVal,
				},
			}

			jsonData, _ := json.Marshal(data)

			// Add to time-series stream
			client.XAdd(fmt.Sprintf("sensor:%s:data", sensorID), "*", "value", string(jsonData))

			// Keep last 24h of data (approx)
			client.XTrim(fmt.Sprintf("sensor:%s:data", sensorID), 17280)

			// Update current value
			client.Set(fmt.Sprintf("sensor:%s:current", sensorID), fmt.Sprintf("%.2f", value))

			// Check thresholds and create alerts
			if value > maxVal || value < minVal {
				alert := Alert{
					ID:           fmt.Sprintf("alert_%d", time.Now().UnixNano()),
					Level:        "warning",
					Message:      fmt.Sprintf("%s %s out of range: %.2f %s", sensor["location"], sensorType, value, sensor["unit"]),
					Source:       sensorID,
					Timestamp:    time.Now().Unix(),
					Acknowledged: false,
				}
				alertJSON, _ := json.Marshal(alert)
				client.XAdd("alerts:stream", "*", "data", string(alertJSON))
				client.LPush("alerts:unacknowledged", string(alertJSON))
			}
		}
	}
}

func monitorAlerts(client *cachestorm.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		// Check unacknowledged alerts
		alerts, _ := client.LRange("alerts:unacknowledged", 0, 9)
		if len(alerts) > 0 {
			fmt.Printf("\n⚠️  %d unacknowledged alerts\n", len(alerts))
		}

		// Auto-acknowledge old alerts
		for _, alertJSON := range alerts {
			var alert Alert
			json.Unmarshal([]byte(alertJSON), &alert)

			if time.Now().Unix()-alert.Timestamp > 300 { // 5 minutes
				client.LRem("alerts:unacknowledged", 0, alertJSON)
				client.LPush("alerts:acknowledged", alertJSON)
			}
		}
	}
}

func runDashboard(client *cachestorm.Client) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	fmt.Println("\n📊 Real-time Dashboard (Press Ctrl+C to exit)")
	fmt.Println(strings.Repeat("=", 60))

	for range ticker.C {
		fmt.Print("\033[H\033[2J") // Clear screen
		fmt.Println("📊 IoT Monitoring Dashboard")
		fmt.Println(strings.Repeat("=", 60))

		// Sensor Status
		fmt.Println("\n🌡️  Sensor Status:")
		sensors, _ := client.SMembers("sensors:active")
		for _, sensorID := range sensors {
			current, _ := client.Get(fmt.Sprintf("sensor:%s:current", sensorID))
			sensorData, _ := client.Get(fmt.Sprintf("sensor:%s", sensorID))
			var sensor map[string]interface{}
			json.Unmarshal([]byte(sensorData), &sensor)

			status := "✅"
			val := 0.0
			fmt.Sscanf(current, "%f", &val)
			if val > sensor["max"].(float64) || val < sensor["min"].(float64) {
				status = "⚠️"
			}

			fmt.Printf("  %s %s (%s): %.2f %s\n",
				status, sensor["location"], sensor["type"], val, sensor["unit"])
		}

		// Recent Alerts
		fmt.Println("\n🔔 Recent Alerts:")
		alertData, _ := client.XRevRange("alerts:stream", "+", "-", 5)
		if len(alertData) == 0 {
			fmt.Println("  No recent alerts")
		} else {
			for _, data := range alertData {
				var alert Alert
				json.Unmarshal([]byte(data), &alert)
				timeStr := time.Unix(alert.Timestamp, 0).Format("15:04:05")
				fmt.Printf("  [%s] %s: %s\n", timeStr, alert.Level, alert.Message)
			}
		}

		// System Stats
		fmt.Println("\n📈 System Stats:")
		totalEvents, _ := client.XLen("sensor:temp_001:data")
		fmt.Printf("  Total sensor readings: %d\n", totalEvents)

		unackAlerts, _ := client.LLen("alerts:unacknowledged")
		fmt.Printf("  Unacknowledged alerts: %d\n", unackAlerts)

		activeSensors, _ := client.SCard("sensors:active")
		fmt.Printf("  Active sensors: %d\n", activeSensors)

		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Println("Last updated:", time.Now().Format("15:04:05"))
	}
}

// Helper variable
var strings = struct {
	Repeat func(string, int) string
}{
	Repeat: func(s string, count int) string {
		result := ""
		for i := 0; i < count; i++ {
			result += s
		}
		return result
	},
}
