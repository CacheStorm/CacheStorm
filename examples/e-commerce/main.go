package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/cachestorm/cachestorm/clients/go/cachestorm"
)

// Product represents an e-commerce product
type Product struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Category    string   `json:"category"`
	Price       float64  `json:"price"`
	Stock       int      `json:"stock"`
	Tags        []string `json:"tags"`
	Description string   `json:"description"`
}

// CartItem represents an item in shopping cart
type CartItem struct {
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

// ShoppingCart represents a user's shopping cart
type ShoppingCart struct {
	UserID int        `json:"user_id"`
	Items  []CartItem `json:"items"`
	Total  float64    `json:"total"`
}

func main() {
	// Connect to CacheStorm
	client, err := cachestorm.New("localhost:6379")
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer client.Close()

	fmt.Println("=== E-Commerce Example with CacheStorm ===")

	// 1. Initialize products
	fmt.Println("\n1. Initializing products...")
	initializeProducts(client)

	// 2. Simulate user browsing
	fmt.Println("\n2. Simulating user browsing...")
	simulateBrowsing(client)

	// 3. Add items to cart
	fmt.Println("\n3. Adding items to cart...")
	userID := 12345
	cart := addToCart(client, userID, 42, 2)
	addToCart(client, userID, 15, 1)

	// 4. View cart
	fmt.Println("\n4. Viewing cart...")
	viewCart(client, userID)

	// 5. Apply discount
	fmt.Println("\n5. Applying discount...")
	applyDiscount(client, userID, 10) // 10% discount

	// 6. Process order
	fmt.Println("\n6. Processing order...")
	processOrder(client, userID, cart)

	// 7. Show analytics
	fmt.Println("\n7. Showing analytics...")
	showAnalytics(client)

	fmt.Println("\n=== E-Commerce Example Complete ===")
}

func initializeProducts(client *cachestorm.Client) {
	categories := []string{"electronics", "clothing", "food", "books"}
	tags := [][]string{
		{"new", "featured"},
		{"sale", "popular"},
		{"bestseller"},
		{"limited"},
	}

	for i := 0; i < 100; i++ {
		product := Product{
			ID:          i,
			Name:        fmt.Sprintf("Product %d", i),
			Category:    categories[rand.Intn(len(categories))],
			Price:       float64(rand.Intn(10000)) / 100.0,
			Stock:       rand.Intn(1000),
			Tags:        tags[rand.Intn(len(tags))],
			Description: fmt.Sprintf("Description for product %d", i),
		}

		data, _ := json.Marshal(product)

		// Store product
		client.Set(fmt.Sprintf("product:%d", i), string(data))

		// Add to category index
		client.SAdd(fmt.Sprintf("category:%s", product.Category), i)

		// Add to price sorted set
		client.ZAdd("products:by_price", product.Price, i)

		// Add tags
		for _, tag := range product.Tags {
			client.SAdd(fmt.Sprintf("tag:%s", tag), i)
		}
	}

	fmt.Println("  - 100 products initialized")
}

func simulateBrowsing(client *cachestorm.Client) {
	// Simulate 10 users browsing
	for userID := 0; userID < 10; userID++ {
		// Record page view
		view := map[string]interface{}{
			"user_id":   userID,
			"page":      "/products",
			"timestamp": time.Now().Unix(),
		}
		viewJSON, _ := json.Marshal(view)
		client.LPush("analytics:page_views", string(viewJSON))

		// Get recommended products (high rated)
		recommended, _ := client.ZRevRange("products:by_price", 0, 4)
		fmt.Printf("  - User %d viewed top 5 expensive products: %v\n", userID, recommended)
	}
}

func addToCart(client *cachestorm.Client, userID, productID, qty int) *ShoppingCart {
	// Get product
	productData, err := client.Get(fmt.Sprintf("product:%d", productID))
	if err != nil {
		log.Printf("Product not found: %d", productID)
		return nil
	}

	var product Product
	json.Unmarshal([]byte(productData), &product)

	// Add to cart
	cartKey := fmt.Sprintf("cart:%d", userID)
	item := CartItem{
		ProductID: productID,
		Quantity:  qty,
		Price:     product.Price,
	}
	itemJSON, _ := json.Marshal(item)
	client.HSet(cartKey, fmt.Sprintf("item:%d", productID), string(itemJSON))

	// Set cart expiry (1 hour)
	client.Expire(cartKey, 3600)

	fmt.Printf("  - Added %d x Product %d to cart\n", qty, productID)
	return nil
}

func viewCart(client *cachestorm.Client, userID int) {
	cartKey := fmt.Sprintf("cart:%d", userID)
	items, _ := client.HGetAll(cartKey)

	var total float64
	fmt.Printf("  - Cart for user %d:\n", userID)
	for field, value := range items {
		var item CartItem
		json.Unmarshal([]byte(value), &item)
		subtotal := float64(item.Quantity) * item.Price
		total += subtotal
		fmt.Printf("    %s: %d x $%.2f = $%.2f\n", field, item.Quantity, item.Price, subtotal)
	}
	fmt.Printf("    Total: $%.2f\n", total)
}

func applyDiscount(client *cachestorm.Client, userID int, discountPercent float64) {
	discountKey := fmt.Sprintf("discount:%d", userID)
	client.Set(discountKey, fmt.Sprintf("%.0f", discountPercent))
	client.Expire(discountKey, 3600)
	fmt.Printf("  - Applied %d%% discount for user %d\n", int(discountPercent), userID)
}

func processOrder(client *cachestorm.Client, userID int, cart *ShoppingCart) {
	// Generate order ID
	orderID := rand.Intn(1000000)
	orderKey := fmt.Sprintf("order:%d", orderID)

	// Get cart items
	cartKey := fmt.Sprintf("cart:%d", userID)
	items, _ := client.HGetAll(cartKey)

	// Calculate total
	var total float64
	for _, value := range items {
		var item CartItem
		json.Unmarshal([]byte(value), &item)
		total += float64(item.Quantity) * item.Price
	}

	// Apply discount if exists
	discountKey := fmt.Sprintf("discount:%d", userID)
	discountData, err := client.Get(discountKey)
	if err == nil && discountData != "" {
		var discount float64
		fmt.Sscanf(discountData, "%f", &discount)
		discountAmount := total * (discount / 100.0)
		total -= discountAmount
		fmt.Printf("  - Discount applied: $%.2f\n", discountAmount)
	}

	// Store order
	order := map[string]interface{}{
		"order_id":   orderID,
		"user_id":    userID,
		"items":      items,
		"total":      total,
		"status":     "confirmed",
		"created_at": time.Now().Format(time.RFC3339),
	}
	orderJSON, _ := json.Marshal(order)
	client.Set(orderKey, string(orderJSON))

	// Clear cart
	client.Del(cartKey)

	// Add to user's order history
	client.SAdd(fmt.Sprintf("user:%d:orders", userID), orderID)

	fmt.Printf("  - Order %d created: $%.2f\n", orderID, total)
}

func showAnalytics(client *cachestorm.Client) {
	// Total products by category
	categories := []string{"electronics", "clothing", "food", "books"}
	fmt.Println("  - Products by category:")
	for _, cat := range categories {
		count, _ := client.SCard(fmt.Sprintf("category:%s", cat))
		fmt.Printf("    %s: %d\n", cat, count)
	}

	// Recent page views
	views, _ := client.LRange("analytics:page_views", 0, 9)
	fmt.Printf("  - Recent page views: %d\n", len(views))

	// Top expensive products
	topProducts, _ := client.ZRevRange("products:by_price", 0, 4)
	fmt.Printf("  - Top 5 expensive products: %v\n", topProducts)
}
