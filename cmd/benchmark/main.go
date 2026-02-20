package main

import (
	"bufio"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
)

type BenchmarkResult struct {
	Name      string
	Ops       int64
	Duration  time.Duration
	OpsPerSec float64
	Latency   time.Duration
	Errors    int64
}

type progressBar struct {
	total     int64
	current   int64
	width     int
	startTime time.Time
}

func newProgressBar(total int64) *progressBar {
	return &progressBar{
		total:     total,
		width:     50,
		startTime: time.Now(),
	}
}

func (p *progressBar) increment() {
	atomic.AddInt64(&p.current, 1)
}

func (p *progressBar) print() {
	current := atomic.LoadInt64(&p.current)
	percent := float64(current) / float64(p.total)
	filled := int(percent * float64(p.width))

	fmt.Printf("\r[")
	for i := 0; i < p.width; i++ {
		if i < filled {
			fmt.Print("█")
		} else {
			fmt.Print("░")
		}
	}
	elapsed := time.Since(p.startTime)
	var remaining time.Duration
	if current > 0 {
		remaining = time.Duration(float64(elapsed) * float64(p.total-current) / float64(current))
	}
	fmt.Printf("] %.1f%% | %d/%d | ETA: %s", percent*100, current, p.total, remaining.Round(time.Second))
}

func runNonInteractive(benchmark, addr string, ops int64, workers int) {
	if !checkServer(addr) {
		fmt.Fprintf(os.Stderr, "Error: Server not running at %s\n", addr)
		os.Exit(1)
	}

	fmt.Printf("Running benchmark: %s\n", benchmark)
	fmt.Printf("Server: %s | Ops: %d | Workers: %d\n\n", addr, ops, workers)

	var results []BenchmarkResult
	var result BenchmarkResult

	switch strings.ToLower(benchmark) {
	case "basic":
		results = runBasicBenchmarks(addr, ops, workers)
		printResults(results)
	case "datatypes":
		results = runDataTypeBenchmarks(addr, ops, workers)
		printResults(results)
	case "realworld":
		results = runRealWorldScenarios(addr, ops, workers)
		printResults(results)
	case "all":
		fmt.Println("=== BASIC BENCHMARKS ===")
		results = runBasicBenchmarks(addr, ops, workers)
		printResults(results)
		fmt.Println("\n=== DATA TYPE BENCHMARKS ===")
		results = runDataTypeBenchmarks(addr, ops, workers)
		printResults(results)
		fmt.Println("\n=== REAL-WORLD SCENARIOS ===")
		results = runRealWorldScenarios(addr, ops, workers)
		printResults(results)
	case "set":
		result = runBenchmark("SET", addr, ops, workers, setOp)
		printSingleResult(result)
	case "get":
		warmup(addr, 1000)
		result = runBenchmark("GET", addr, ops, workers, getOp)
		printSingleResult(result)
	case "hash":
		result = runBenchmark("HASH (HSET/HGET)", addr, ops, workers, hashOp)
		printSingleResult(result)
	case "list":
		result = runBenchmark("LIST (LPUSH/RPOP)", addr, ops, workers, listOp)
		printSingleResult(result)
	case "setop":
		result = runBenchmark("SET (SADD/SISMEMBER)", addr, ops, workers, setOp2)
		printSingleResult(result)
	case "zset":
		result = runBenchmark("ZSET (ZADD/ZSCORE)", addr, ops, workers, zsetOp)
		printSingleResult(result)
	case "pipeline":
		result = runPipelineBenchmark(addr, ops, 10)
		printSingleResult(result)
	case "tag":
		result = runTagBenchmark(addr, ops, workers)
		printSingleResult(result)
	case "session":
		result = runUserSessionScenario(addr, ops, workers)
		printSingleResult(result)
	case "cart":
		result = runShoppingCartScenario(addr, ops, workers)
		printSingleResult(result)
	case "leaderboard":
		result = runLeaderboardScenario(addr, ops, workers)
		printSingleResult(result)
	case "ratelimit":
		result = runRateLimiterScenario(addr, ops, workers)
		printSingleResult(result)
	default:
		fmt.Fprintf(os.Stderr, "Unknown benchmark: %s\nUse -list to see available benchmarks\n", benchmark)
		os.Exit(1)
	}
}

func main() {
	runFlag := flag.String("run", "", "Run specific benchmark: basic, datatypes, realworld, set, get, hash, list, setop, zset, pipeline, tag, session, cart, leaderboard, ratelimit, all")
	opsFlag := flag.Int("ops", 100000, "Number of operations")
	workersFlag := flag.Int("workers", runtime.NumCPU(), "Number of workers")
	addrFlag := flag.String("addr", "127.0.0.1:6380", "Server address")
	listFlag := flag.Bool("list", false, "List available benchmarks")
	flag.Parse()

	if *listFlag {
		fmt.Println("Available benchmarks:")
		fmt.Println("  basic       - Run all basic benchmarks (SET, GET, INCR, MIXED)")
		fmt.Println("  datatypes   - Run all data type benchmarks (Hash, List, Set, ZSet)")
		fmt.Println("  realworld   - Run all real-world scenarios")
		fmt.Println("  set         - SET benchmark only")
		fmt.Println("  get         - GET benchmark only")
		fmt.Println("  hash        - Hash operations benchmark")
		fmt.Println("  list        - List operations benchmark")
		fmt.Println("  setop       - Set operations benchmark")
		fmt.Println("  zset        - Sorted set benchmark")
		fmt.Println("  pipeline    - Pipeline benchmark")
		fmt.Println("  tag         - Tag operations benchmark")
		fmt.Println("  session     - User session scenario")
		fmt.Println("  cart        - Shopping cart scenario")
		fmt.Println("  leaderboard - Leaderboard scenario")
		fmt.Println("  ratelimit   - Rate limiter scenario")
		fmt.Println("  all         - Run all benchmarks")
		return
	}

	if *runFlag != "" {
		runNonInteractive(*runFlag, *addrFlag, int64(*opsFlag), *workersFlag)
		return
	}

	color.Cyan(`
   ██████╗  █████╗ ███████╗████████╗██████╗  ██████╗ ███████╗
  ██╔════╝ ██╔══██╗██╔════╝╚══██╔╝██╔══██╗██╔════╝ ██╔════╝
  ██║       ███████║███████╗   ██║   ██████╔╝██║  ███╗█████╗  
  ██║       ██╔══██║╚════██║   ██║   ██╔══██╗██║   ██║██╔══╝  
  ╚██████╗  ██║  ██║███████║   ██║   ██║  ██║╚██████╔╝███████╗
   ╚═════╝  ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
  
  Benchmark Tool v2.0.0
`)

	var serverCmd *exec.Cmd
	var serverRunning bool

	cleanup := func() {
		if serverCmd != nil && serverCmd.Process != nil {
			color.Yellow("\nStopping server...")
			serverCmd.Process.Kill()
		}
	}
	defer cleanup()

	addr := "127.0.0.1:6380"
	ops := int64(100000)
	workers := runtime.NumCPU()

	for {
		fmt.Println()
		color.White("┌─────────────────────────────────────────┐")
		color.White("│           BENCHMARK MENU                │")
		color.White("├─────────────────────────────────────────┤")
		color.White("│  1. Start Server                        │")
		color.White("│  2. Run All Basic Benchmarks            │")
		color.White("│  3. Run All Data Type Benchmarks        │")
		color.White("│  4. Run Real-World Scenarios            │")
		color.White("│  ─────────────────────────────────────  │")
		color.White("│  5. SET Benchmark                       │")
		color.White("│  6. GET Benchmark                       │")
		color.White("│  7. Hash Operations Benchmark           │")
		color.White("│  8. List Operations Benchmark           │")
		color.White("│  9. Set Operations Benchmark            │")
		color.White("│  10. Sorted Set Benchmark               │")
		color.White("│  11. Pipeline Benchmark                 │")
		color.White("│  12. Tag Operations Benchmark           │")
		color.White("│  ─────────────────────────────────────  │")
		color.White("│  13. User Session Scenario              │")
		color.White("│  14. Shopping Cart Scenario             │")
		color.White("│  15. Leaderboard Scenario               │")
		color.White("│  16. Rate Limiter Scenario              │")
		color.White("│  ─────────────────────────────────────  │")
		color.White("│  17. Configure (ops=%d, workers=%d)   │", ops, workers)
		color.White("│  18. Show System Info                   │")
		color.White("│  0. Exit                                │")
		color.White("└─────────────────────────────────────────┘")

		choice := readInput("Select option: ")
		fmt.Println()

		switch choice {
		case "1":
			if serverRunning {
				color.Yellow("Server already running")
				break
			}
			color.Green("Starting CacheStorm server...")
			serverCmd = exec.Command("./cachestorm.exe", "-port", "6380")
			serverCmd.Stdout = os.Stdout
			serverCmd.Stderr = os.Stderr
			if err := serverCmd.Start(); err != nil {
				color.Red("Failed to start server: %v", err)
				break
			}
			time.Sleep(500 * time.Millisecond)
			serverRunning = true
			color.Green("Server started on %s", addr)

		case "2":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			results := runBasicBenchmarks(addr, ops, workers)
			printResults(results)

		case "3":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			results := runDataTypeBenchmarks(addr, ops, workers)
			printResults(results)

		case "4":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			results := runRealWorldScenarios(addr, ops, workers)
			printResults(results)

		case "5":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runBenchmark("SET", addr, ops, workers, setOp)
			printSingleResult(result)

		case "6":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			warmup(addr, 1000)
			result := runBenchmark("GET", addr, ops, workers, getOp)
			printSingleResult(result)

		case "7":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runBenchmark("HASH (HSET/HGET)", addr, ops, workers, hashOp)
			printSingleResult(result)

		case "8":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runBenchmark("LIST (LPUSH/RPOP)", addr, ops, workers, listOp)
			printSingleResult(result)

		case "9":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runBenchmark("SET (SADD/SISMEMBER)", addr, ops, workers, setOp2)
			printSingleResult(result)

		case "10":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runBenchmark("ZSET (ZADD/ZSCORE)", addr, ops, workers, zsetOp)
			printSingleResult(result)

		case "11":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runPipelineBenchmark(addr, ops, 10)
			printSingleResult(result)

		case "12":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runTagBenchmark(addr, ops, workers)
			printSingleResult(result)

		case "13":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runUserSessionScenario(addr, ops, workers)
			printSingleResult(result)

		case "14":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runShoppingCartScenario(addr, ops, workers)
			printSingleResult(result)

		case "15":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runLeaderboardScenario(addr, ops, workers)
			printSingleResult(result)

		case "16":
			if !checkServer(addr) {
				color.Red("Server not running!")
				break
			}
			result := runRateLimiterScenario(addr, ops, workers)
			printSingleResult(result)

		case "17":
			opsStr := readInput(fmt.Sprintf("Operations [%d]: ", ops))
			if opsStr != "" {
				fmt.Sscanf(opsStr, "%d", &ops)
			}
			workersStr := readInput(fmt.Sprintf("Workers [%d]: ", workers))
			if workersStr != "" {
				fmt.Sscanf(workersStr, "%d", &workers)
			}
			color.Green("Configuration updated")

		case "18":
			printSystemInfo()

		case "0":
			color.Yellow("Goodbye!")
			return

		default:
			color.Red("Invalid option")
		}
	}
}

func readInput(prompt string) string {
	fmt.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

func checkServer(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, time.Second)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func sendRespCommand(conn net.Conn, args ...string) (string, error) {
	cmd := fmt.Sprintf("*%d\r\n", len(args))
	for _, arg := range args {
		cmd += fmt.Sprintf("$%d\r\n%s\r\n", len(arg), arg)
	}

	if _, err := conn.Write([]byte(cmd)); err != nil {
		return "", err
	}

	reader := bufio.NewReader(conn)
	return readRespResponse(reader)
}

func readRespResponse(reader *bufio.Reader) (string, error) {
	var result strings.Builder

	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	result.WriteString(line)

	if len(line) == 0 {
		return result.String(), nil
	}

	switch line[0] {
	case '+', '-', ':', '_', '#', ',', '(':
		return result.String(), nil
	case '!':
		fallthrough
	case '=':
		sizeStr := strings.TrimSpace(line[1:])
		var size int
		fmt.Sscanf(sizeStr, "%d", &size)
		if size >= 0 {
			data := make([]byte, size+2)
			if _, err := io.ReadFull(reader, data); err != nil {
				return "", err
			}
			result.Write(data)
		}
		return result.String(), nil
	case '$':
		sizeStr := strings.TrimSpace(line[1:])
		var size int
		fmt.Sscanf(sizeStr, "%d", &size)
		if size >= 0 {
			data := make([]byte, size+2)
			if _, err := io.ReadFull(reader, data); err != nil {
				return "", err
			}
			result.Write(data)
		}
		return result.String(), nil
	case '*':
		countStr := strings.TrimSpace(line[1:])
		var count int
		fmt.Sscanf(countStr, "%d", &count)
		if count > 0 {
			for i := 0; i < count; i++ {
				elem, err := readRespResponse(reader)
				if err != nil {
					return "", err
				}
				result.WriteString(elem)
			}
		}
		return result.String(), nil
	case '%', '~':
		countStr := strings.TrimSpace(line[1:])
		var count int
		fmt.Sscanf(countStr, "%d", &count)
		for i := 0; i < count; i++ {
			elem, err := readRespResponse(reader)
			if err != nil {
				return "", err
			}
			result.WriteString(elem)
		}
		return result.String(), nil
	}
	return result.String(), nil
}

func randomID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func warmup(addr string, count int) int64 {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return 0
	}
	defer conn.Close()

	for i := 0; i < count; i++ {
		sendRespCommand(conn, "SET", fmt.Sprintf("warmup%d", i), fmt.Sprintf("%d", i))
	}
	return int64(count)
}

type opFunc func(conn net.Conn, i int64) error

func runBenchmark(name, addr string, totalOps int64, workers int, op opFunc) BenchmarkResult {
	var ops atomic.Int64
	var errors atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	opsPerWorker := totalOps / int64(workers)

	progress := newProgressBar(totalOps)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(100 * time.Millisecond):
				progress.print()
			}
		}
	}()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int64) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(opsPerWorker)
				return
			}
			defer conn.Close()

			for i := int64(0); i < opsPerWorker; i++ {
				if err := op(conn, workerID*opsPerWorker+i); err != nil {
					errors.Add(1)
				}
				ops.Add(1)
				progress.increment()
			}
		}(int64(w))
	}

	wg.Wait()
	close(done)
	fmt.Println()

	duration := time.Since(start)
	totalOpsDone := ops.Load()
	totalErrors := errors.Load()

	var latency time.Duration
	if totalOpsDone > 0 {
		latency = duration / time.Duration(totalOpsDone)
	}

	return BenchmarkResult{
		Name:      name,
		Ops:       totalOpsDone,
		Duration:  duration,
		OpsPerSec: float64(totalOpsDone) / duration.Seconds(),
		Latency:   latency,
		Errors:    totalErrors,
	}
}

// Basic Operations
func setOp(conn net.Conn, i int64) error {
	_, err := sendRespCommand(conn, "SET", fmt.Sprintf("key:%d", i%10000), fmt.Sprintf("value%d", i))
	return err
}

func getOp(conn net.Conn, i int64) error {
	_, err := sendRespCommand(conn, "GET", fmt.Sprintf("key:%d", i%10000))
	return err
}

// Hash Operations
func hashOp(conn net.Conn, i int64) error {
	key := fmt.Sprintf("user:%d", i%1000)
	if i%2 == 0 {
		_, err := sendRespCommand(conn, "HSET", key, "name", fmt.Sprintf("User%d", i), "email", fmt.Sprintf("user%d@test.com", i), "age", fmt.Sprintf("%d", 20+i%50))
		return err
	}
	_, err := sendRespCommand(conn, "HGET", key, "name")
	return err
}

// List Operations
func listOp(conn net.Conn, i int64) error {
	key := fmt.Sprintf("queue:%d", i%100)
	if i%2 == 0 {
		_, err := sendRespCommand(conn, "LPUSH", key, fmt.Sprintf("task%d", i))
		return err
	}
	_, err := sendRespCommand(conn, "RPOP", key)
	return err
}

// Set Operations
func setOp2(conn net.Conn, i int64) error {
	key := fmt.Sprintf("set:%d", i%100)
	if i%2 == 0 {
		_, err := sendRespCommand(conn, "SADD", key, fmt.Sprintf("member%d", i%1000))
		return err
	}
	_, err := sendRespCommand(conn, "SISMEMBER", key, fmt.Sprintf("member%d", i%1000))
	return err
}

// Sorted Set Operations
func zsetOp(conn net.Conn, i int64) error {
	key := fmt.Sprintf("leaderboard:%d", i%10)
	if i%2 == 0 {
		_, err := sendRespCommand(conn, "ZADD", key, fmt.Sprintf("%d", i%10000), fmt.Sprintf("player%d", i%1000))
		return err
	}
	_, err := sendRespCommand(conn, "ZSCORE", key, fmt.Sprintf("player%d", i%1000))
	return err
}

// Tag Operations
func tagOp(conn net.Conn, i int64) error {
	key := fmt.Sprintf("cached:%d", i%5000)
	tag := fmt.Sprintf("page:%d", i%100)
	_, err := sendRespCommand(conn, "SET", key, fmt.Sprintf("data%d", i), "TAGS", tag)
	return err
}

func runBasicBenchmarks(addr string, ops int64, workers int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0)

	color.Cyan("Running warmup...")
	warmup(addr, 5000)

	benchmarks := []struct {
		name string
		fn   opFunc
	}{
		{"SET", setOp},
		{"GET", getOp},
		{"INCR", incrOp},
		{"MIXED R/W (80% READ)", mixedOp},
	}

	for _, b := range benchmarks {
		color.Cyan("\nRunning %s benchmark...", b.name)
		result := runBenchmark(b.name, addr, ops, workers, b.fn)
		results = append(results, result)
	}

	return results
}

func runDataTypeBenchmarks(addr string, ops int64, workers int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0)

	benchmarks := []struct {
		name string
		fn   opFunc
	}{
		{"STRING (SET/GET)", setOp},
		{"HASH (HSET/HGET)", hashOp},
		{"LIST (LPUSH/RPOP)", listOp},
		{"SET (SADD/SISMEMBER)", setOp2},
		{"ZSET (ZADD/ZSCORE)", zsetOp},
	}

	for _, b := range benchmarks {
		color.Cyan("\nRunning %s benchmark...", b.name)
		result := runBenchmark(b.name, addr, ops, workers, b.fn)
		results = append(results, result)
	}

	return results
}

func runRealWorldScenarios(addr string, ops int64, workers int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0)

	scenarios := []struct {
		name string
		fn   opFunc
	}{
		{"User Session Management", userSessionOp},
		{"Shopping Cart Operations", cartOp},
		{"Leaderboard Updates", leaderboardOp},
		{"Rate Limiter Checks", rateLimitOp},
		{"Cache-Aside Pattern", cacheAsideOp},
		{"Pub/Sub Messaging", pubsubOp},
	}

	for _, s := range scenarios {
		color.Cyan("\nRunning %s scenario...", s.name)
		result := runBenchmark(s.name, addr, ops, workers, s.fn)
		results = append(results, result)
	}

	return results
}

func incrOp(conn net.Conn, i int64) error {
	_, err := sendRespCommand(conn, "INCR", fmt.Sprintf("counter:%d", i%100))
	return err
}

func mixedOp(conn net.Conn, i int64) error {
	if i%5 == 0 {
		_, err := sendRespCommand(conn, "SET", fmt.Sprintf("mix:%d", i%10000), fmt.Sprintf("val%d", i))
		return err
	}
	_, err := sendRespCommand(conn, "GET", fmt.Sprintf("mix:%d", i%10000))
	return err
}

// Real-World Scenario Operations

func userSessionOp(conn net.Conn, i int64) error {
	sessionID := fmt.Sprintf("sess:%s", randomID())
	userID := i % 10000

	switch i % 5 {
	case 0:
		_, err := sendRespCommand(conn, "HSET", sessionID, "user_id", fmt.Sprintf("%d", userID), "created", fmt.Sprintf("%d", time.Now().Unix()), "ip", "192.168.1.1")
		return err
	case 1:
		_, err := sendRespCommand(conn, "EXPIRE", sessionID, "3600")
		return err
	case 2:
		_, err := sendRespCommand(conn, "HGET", sessionID, "user_id")
		return err
	case 3:
		_, err := sendRespCommand(conn, "TTL", sessionID)
		return err
	default:
		_, err := sendRespCommand(conn, "DEL", sessionID)
		return err
	}
}

func cartOp(conn net.Conn, i int64) error {
	cartID := fmt.Sprintf("cart:%d", i%1000)
	productID := i % 10000
	qty := (i % 5) + 1

	switch i % 4 {
	case 0:
		_, err := sendRespCommand(conn, "HSET", cartID, fmt.Sprintf("product:%d", productID), fmt.Sprintf("%d", qty))
		return err
	case 1:
		_, err := sendRespCommand(conn, "HGET", cartID, fmt.Sprintf("product:%d", productID))
		return err
	case 2:
		_, err := sendRespCommand(conn, "HLEN", cartID)
		return err
	default:
		_, err := sendRespCommand(conn, "HDEL", cartID, fmt.Sprintf("product:%d", productID))
		return err
	}
}

func leaderboardOp(conn net.Conn, i int64) error {
	board := "lb:global"
	playerID := i % 1000
	score := i % 10000

	_, err := sendRespCommand(conn, "ZADD", board, fmt.Sprintf("%d", score), fmt.Sprintf("player:%d", playerID))
	return err
}

func rateLimitOp(conn net.Conn, i int64) error {
	apiKey := fmt.Sprintf("api:%d", i%100)
	window := time.Now().Unix() / 60
	key := fmt.Sprintf("rl:%s:%d", apiKey, window)

	_, err := sendRespCommand(conn, "INCR", key)
	if err != nil {
		return err
	}
	_, err = sendRespCommand(conn, "EXPIRE", key, "60")
	return err
}

func cacheAsideOp(conn net.Conn, i int64) error {
	key := fmt.Sprintf("db:users:%d", i%5000)

	_, err := sendRespCommand(conn, "GET", key)
	if err != nil {
		return err
	}

	if i%10 == 0 {
		_, err = sendRespCommand(conn, "SET", key, fmt.Sprintf("{\"id\":%d,\"name\":\"User\"}", i%5000), "EX", "300")
	}
	return err
}

func pubsubOp(conn net.Conn, i int64) error {
	channel := fmt.Sprintf("channel:%d", i%10)
	message := fmt.Sprintf("msg:%d:%d", i, time.Now().UnixNano())

	_, err := sendRespCommand(conn, "PUBLISH", channel, message)
	return err
}

func runTagBenchmark(addr string, ops int64, workers int) BenchmarkResult {
	color.Cyan("Running tag benchmark...")

	var totalOps atomic.Int64
	var errors atomic.Int64
	var wg sync.WaitGroup

	start := time.Now()
	opsPerWorker := ops / int64(workers)

	progress := newProgressBar(ops)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(100 * time.Millisecond):
				progress.print()
			}
		}
	}()

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(workerID int64) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(opsPerWorker)
				return
			}
			defer conn.Close()

			for i := int64(0); i < opsPerWorker; i++ {
				idx := workerID*opsPerWorker + i
				var cmdErr error

				switch i % 4 {
				case 0:
					key := fmt.Sprintf("tagged:%d", idx%5000)
					tag := fmt.Sprintf("page:%d", idx%100)
					_, cmdErr = sendRespCommand(conn, "SET", key, fmt.Sprintf("data%d", idx), "TAGS", tag)
				case 1:
					tag := fmt.Sprintf("page:%d", idx%100)
					_, cmdErr = sendRespCommand(conn, "TAGKEYS", tag)
				case 2:
					tag := fmt.Sprintf("page:%d", idx%100)
					_, cmdErr = sendRespCommand(conn, "TAGCOUNT", tag)
				case 3:
					_, cmdErr = sendRespCommand(conn, "TAGS")
				}

				if cmdErr != nil {
					errors.Add(1)
				}
				totalOps.Add(1)
				progress.increment()
			}
		}(int64(w))
	}

	wg.Wait()
	close(done)
	fmt.Println()

	duration := time.Since(start)
	opsDone := totalOps.Load()

	return BenchmarkResult{
		Name:      "TAG OPERATIONS",
		Ops:       opsDone,
		Duration:  duration,
		OpsPerSec: float64(opsDone) / duration.Seconds(),
		Latency:   duration / time.Duration(opsDone),
		Errors:    errors.Load(),
	}
}

func runPipelineBenchmark(addr string, ops int64, pipelineSize int) BenchmarkResult {
	color.Cyan("Running warmup...")
	warmup(addr, 1000)

	var totalOps atomic.Int64
	var errors atomic.Int64

	start := time.Now()
	batches := ops / int64(pipelineSize)

	progress := newProgressBar(batches)
	done := make(chan bool)
	go func() {
		for {
			select {
			case <-done:
				return
			case <-time.After(100 * time.Millisecond):
				progress.print()
			}
		}
	}()

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return BenchmarkResult{Name: "PIPELINE", Errors: 1}
	}
	defer conn.Close()

	for b := int64(0); b < batches; b++ {
		for i := 0; i < pipelineSize; i++ {
			idx := b*int64(pipelineSize) + int64(i)
			switch i % 4 {
			case 0:
				sendRespCommand(conn, "SET", fmt.Sprintf("pipe:%d", idx%10000), fmt.Sprintf("val%d", idx))
			case 1:
				sendRespCommand(conn, "GET", fmt.Sprintf("pipe:%d", idx%10000))
			case 2:
				sendRespCommand(conn, "INCR", fmt.Sprintf("counter:%d", idx%100))
			case 3:
				sendRespCommand(conn, "HSET", fmt.Sprintf("hash:%d", idx%1000), "field", fmt.Sprintf("%d", idx))
			}
		}

		reader := bufio.NewReader(conn)
		for i := 0; i < pipelineSize; i++ {
			line, err := reader.ReadString('\n')
			if err != nil || (len(line) > 0 && line[0] == '-') {
				errors.Add(1)
			}
			if len(line) > 0 && (line[0] == '$' || line[0] == '*') {
				reader.ReadString('\n')
			}
		}
		totalOps.Add(int64(pipelineSize))
		progress.increment()
	}
	close(done)
	fmt.Println()

	duration := time.Since(start)
	opsDone := totalOps.Load()

	return BenchmarkResult{
		Name:      fmt.Sprintf("PIPELINE (batch=%d)", pipelineSize),
		Ops:       opsDone,
		Duration:  duration,
		OpsPerSec: float64(opsDone) / duration.Seconds(),
		Latency:   duration / time.Duration(batches),
		Errors:    errors.Load(),
	}
}

func runUserSessionScenario(addr string, ops int64, workers int) BenchmarkResult {
	return runBenchmark("USER SESSION SCENARIO", addr, ops, workers, userSessionOp)
}

func runShoppingCartScenario(addr string, ops int64, workers int) BenchmarkResult {
	return runBenchmark("SHOPPING CART SCENARIO", addr, ops, workers, cartOp)
}

func runLeaderboardScenario(addr string, ops int64, workers int) BenchmarkResult {
	return runBenchmark("LEADERBOARD SCENARIO", addr, ops, workers, leaderboardOp)
}

func runRateLimiterScenario(addr string, ops int64, workers int) BenchmarkResult {
	return runBenchmark("RATE LIMITER SCENARIO", addr, ops, workers, rateLimitOp)
}

func printResults(results []BenchmarkResult) {
	fmt.Println()
	color.Cyan("╔══════════════════════════════════════════════════════════════════════════════════╗")
	color.Cyan("║                               BENCHMARK RESULTS                                   ║")
	color.Cyan("╠══════════════════════════════════════════════════════════════════════════════════╣")
	color.Cyan("║ %-28s │ %12s │ %12s │ %10s │ %6s ║", "BENCHMARK", "OPS/SEC", "LATENCY", "TOTAL OPS", "ERRORS")
	color.Cyan("╠══════════════════════════════════════════════════════════════════════════════════╣")

	sort.Slice(results, func(i, j int) bool {
		return results[i].OpsPerSec > results[j].OpsPerSec
	})

	for _, r := range results {
		status := color.GreenString
		if r.Errors > 0 {
			status = color.RedString
		} else if r.OpsPerSec < 100000 {
			status = color.YellowString
		}
		fmt.Println(status("║ %-28s │ %12.0f │ %12s │ %10d │ %6d ║",
			r.Name, r.OpsPerSec, r.Latency.Round(time.Microsecond), r.Ops, r.Errors))
	}
	color.Cyan("╚══════════════════════════════════════════════════════════════════════════════════╝")

	if len(results) > 0 {
		best := results[0]
		color.Green("\n🏆 Best: %s - %.0f ops/sec", best.Name, best.OpsPerSec)

		var totalOps int64
		var totalErrs int64
		for _, r := range results {
			totalOps += r.Ops
			totalErrs += r.Errors
		}

		if totalErrs == 0 {
			color.Green("✅ All operations successful - 100%% success rate")
		} else {
			successRate := float64(totalOps-totalErrs) / float64(totalOps) * 100
			color.Yellow("⚠️  Success rate: %.2f%% (%d/%d operations)", successRate, totalOps-totalErrs, totalOps)
		}
	}
}

func printSingleResult(r BenchmarkResult) {
	fmt.Println()
	color.Cyan("╔════════════════════════════════════════════════════════════╗")
	color.Cyan("║                   BENCHMARK RESULT                          ║")
	color.Cyan("╠════════════════════════════════════════════════════════════╣")

	status := "✅ PASS"
	statusColor := color.GreenString
	if r.Errors > 0 {
		status = "❌ FAIL"
		statusColor = color.RedString
	}

	fmt.Printf("║ %-25s %-35s ║\n", "Benchmark:", color.WhiteString(r.Name))
	fmt.Printf("║ %-25s %-35s ║\n", "Status:", statusColor(status))
	fmt.Printf("║ %-25s %-35s ║\n", "Total Operations:", color.YellowString("%d", r.Ops))
	fmt.Printf("║ %-25s %-35s ║\n", "Duration:", color.YellowString("%v", r.Duration.Round(time.Millisecond)))
	fmt.Printf("║ %-25s %-35s ║\n", "Throughput:", color.GreenString("%.0f ops/sec", r.OpsPerSec))
	fmt.Printf("║ %-25s %-35s ║\n", "Avg Latency:", color.YellowString("%v", r.Latency.Round(time.Microsecond)))
	fmt.Printf("║ %-25s %-35s ║\n", "Errors:", color.RedString("%d", r.Errors))

	successRate := float64(r.Ops-r.Errors) / float64(r.Ops) * 100
	fmt.Printf("║ %-25s %-35s ║\n", "Success Rate:", color.GreenString("%.2f%%", successRate))

	color.Cyan("╚════════════════════════════════════════════════════════════╝")

	if r.OpsPerSec > 1000000 {
		color.Green("\n🔥 Excellent performance! (> 1M ops/sec)")
	} else if r.OpsPerSec > 500000 {
		color.Green("\n🚀 Great performance! (> 500K ops/sec)")
	} else if r.OpsPerSec > 100000 {
		color.Yellow("\n👍 Good performance (> 100K ops/sec)")
	} else {
		color.Red("\n⚠️  Performance below expectations (< 100K ops/sec)")
	}
}

func printSystemInfo() {
	fmt.Println()
	color.Cyan("╔════════════════════════════════════════════════════════════╗")
	color.Cyan("║                      SYSTEM INFO                            ║")
	color.Cyan("╠════════════════════════════════════════════════════════════╣")
	fmt.Printf("║ %-25s %-35s ║\n", "OS:", runtime.GOOS)
	fmt.Printf("║ %-25s %-35s ║\n", "Arch:", runtime.GOARCH)
	fmt.Printf("║ %-25s %-35s ║\n", "CPU Cores:", fmt.Sprintf("%d", runtime.NumCPU()))
	fmt.Printf("║ %-25s %-35s ║\n", "Go Version:", runtime.Version())
	color.Cyan("╚════════════════════════════════════════════════════════════╝")
}
