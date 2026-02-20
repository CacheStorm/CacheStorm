package main

import (
	"bufio"
	"fmt"
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

func main() {
	color.Cyan(`
   ██████╗  █████╗ ███████╗████████╗██████╗  ██████╗ ███████╗
  ██╔════╝ ██╔══██╗██╔════╝╚══██╔══╝██╔══██╗██╔════╝ ██╔════╝
  ██║       ███████║███████╗   ██║   ██████╔╝██║  ███╗█████╗  
  ██║       ██╔══██║╚════██║   ██║   ██╔══██╗██║   ██║██╔══╝  
  ╚██████╗  ██║  ██║███████║   ██║   ██║  ██║╚██████╔╝███████╗
   ╚═════╝  ╚═╝  ╚═╝╚══════╝   ╚═╝   ╚═╝  ╚═╝ ╚═════╝ ╚══════╝
  
  Benchmark Tool v1.0.0
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
		color.White("│  2. Run All Benchmarks                  │")
		color.White("│  3. Run SET Benchmark                   │")
		color.White("│  4. Run GET Benchmark                   │")
		color.White("│  5. Run Mixed Benchmark                 │")
		color.White("│  6. Run Pipeline Benchmark              │")
		color.White("│  7. Run Concurrent Benchmark            │")
		color.White("│  8. Configure (ops=%d, workers=%d)     │", ops, workers)
		color.White("│  9. Show System Info                    │")
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
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			results := runAllBenchmarks(addr, ops, workers)
			printResults(results)

		case "3":
			if !checkServer(addr) {
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			result := runBenchmark("SET", addr, ops, workers, setOp)
			printSingleResult(result)

		case "4":
			if !checkServer(addr) {
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			_ = warmup(addr)
			result := runBenchmark("GET", addr, ops, workers, getOp)
			printSingleResult(result)

		case "5":
			if !checkServer(addr) {
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			result := runMixedBenchmark(addr, ops, workers)
			printSingleResult(result)

		case "6":
			if !checkServer(addr) {
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			result := runPipelineBenchmark(addr, ops, 10)
			printSingleResult(result)

		case "7":
			if !checkServer(addr) {
				color.Red("Server not running. Start server first (option 1)")
				break
			}
			result := runConcurrentBenchmark(addr, ops, workers)
			printSingleResult(result)

		case "8":
			opsStr := readInput(fmt.Sprintf("Operations [%d]: ", ops))
			if opsStr != "" {
				fmt.Sscanf(opsStr, "%d", &ops)
			}
			workersStr := readInput(fmt.Sprintf("Workers [%d]: ", workers))
			if workersStr != "" {
				fmt.Sscanf(workersStr, "%d", &workers)
			}
			color.Green("Configuration updated")

		case "9":
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

func sendCommand(conn net.Conn, cmd string) (string, error) {
	_, err := conn.Write([]byte(cmd + "\r\n"))
	if err != nil {
		return "", err
	}
	reader := bufio.NewReader(conn)
	resp, err := reader.ReadString('\n')
	return resp, err
}

func warmup(addr string) int64 {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return 0
	}
	defer conn.Close()

	for i := 0; i < 1000; i++ {
		sendCommand(conn, fmt.Sprintf("*3\r\n$3\r\nSET\r\n$6\r\nwarmup\r\n$1\r\n%d\r\n", i))
	}
	return 1000
}

func setOp(conn net.Conn, i int64) error {
	cmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$6\r\nkey:%d\r\n$6\r\nvalue%d\r\n", i%10000, i%10000)
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	_, err = reader.ReadString('\n')
	return err
}

func getOp(conn net.Conn, i int64) error {
	cmd := fmt.Sprintf("*2\r\n$3\r\nGET\r\n$6\r\nkey:%d\r\n", i%10000)
	_, err := conn.Write([]byte(cmd))
	if err != nil {
		return err
	}
	reader := bufio.NewReader(conn)
	_, err = reader.ReadString('\n')
	return err
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
				errors.Add(1)
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

func runAllBenchmarks(addr string, ops int64, workers int) []BenchmarkResult {
	results := make([]BenchmarkResult, 0)

	color.Cyan("Running warmup...")
	warmup(addr)

	benchmarks := []struct {
		name string
		fn   opFunc
	}{
		{"SET", setOp},
		{"GET", getOp},
	}

	for _, b := range benchmarks {
		color.Cyan("\nRunning %s benchmark...", b.name)
		result := runBenchmark(b.name, addr, ops, workers, b.fn)
		results = append(results, result)
	}

	return results
}

func runMixedBenchmark(addr string, ops int64, workers int) BenchmarkResult {
	color.Cyan("Running warmup...")
	warmup(addr)

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
				errors.Add(1)
				return
			}
			defer conn.Close()

			for i := int64(0); i < opsPerWorker; i++ {
				var cmd string
				if i%2 == 0 {
					cmd = fmt.Sprintf("*3\r\n$3\r\nSET\r\n$6\r\nmix%d\r\n$4\r\ndata\r\n", (workerID*opsPerWorker+i)%10000)
				} else {
					cmd = fmt.Sprintf("*2\r\n$3\r\nGET\r\n$6\r\nmix%d\r\n", (workerID*opsPerWorker+i)%10000)
				}
				if _, err := conn.Write([]byte(cmd)); err != nil {
					errors.Add(1)
				} else {
					reader := bufio.NewReader(conn)
					if _, err := reader.ReadString('\n'); err != nil {
						errors.Add(1)
					}
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
		Name:      "MIXED (50% SET, 50% GET)",
		Ops:       opsDone,
		Duration:  duration,
		OpsPerSec: float64(opsDone) / duration.Seconds(),
		Latency:   duration / time.Duration(opsDone),
		Errors:    errors.Load(),
	}
}

func runPipelineBenchmark(addr string, ops int64, pipelineSize int) BenchmarkResult {
	color.Cyan("Running warmup...")
	warmup(addr)

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
			cmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$6\r\npipe%d\r\n$4\r\ndata\r\n", (b*int64(pipelineSize)+int64(i))%10000)
			conn.Write([]byte(cmd))
		}
		reader := bufio.NewReader(conn)
		for i := 0; i < pipelineSize; i++ {
			if _, err := reader.ReadString('\n'); err != nil {
				errors.Add(1)
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

func runConcurrentBenchmark(addr string, ops int64, workers int) BenchmarkResult {
	color.Cyan("Running warmup...")
	warmup(addr)

	var totalOps atomic.Int64
	var errors atomic.Int64
	var wg sync.WaitGroup
	var latencies sync.Map

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
		go func(workerID int) {
			defer wg.Done()
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				errors.Add(1)
				return
			}
			defer conn.Close()

			for i := int64(0); i < opsPerWorker; i++ {
				opStart := time.Now()
				cmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$6\r\nconc%d\r\n$4\r\ndata\r\n", (int64(workerID)*opsPerWorker+i)%10000)
				if _, err := conn.Write([]byte(cmd)); err != nil {
					errors.Add(1)
					continue
				}
				reader := bufio.NewReader(conn)
				if _, err := reader.ReadString('\n'); err != nil {
					errors.Add(1)
					continue
				}
				latencies.Store(workerID, time.Since(opStart))
				totalOps.Add(1)
				progress.increment()
			}
		}(w)
	}

	wg.Wait()
	close(done)
	fmt.Println()

	duration := time.Since(start)
	opsDone := totalOps.Load()

	return BenchmarkResult{
		Name:      fmt.Sprintf("CONCURRENT (workers=%d)", workers),
		Ops:       opsDone,
		Duration:  duration,
		OpsPerSec: float64(opsDone) / duration.Seconds(),
		Latency:   duration / time.Duration(opsDone),
		Errors:    errors.Load(),
	}
}

func printResults(results []BenchmarkResult) {
	fmt.Println()
	color.Cyan("╔══════════════════════════════════════════════════════════════════════════════╗")
	color.Cyan("║                              BENCHMARK RESULTS                                ║")
	color.Cyan("╠══════════════════════════════════════════════════════════════════════════════╣")
	color.Cyan("║ %-20s │ %12s │ %12s │ %10s │ %6s ║", "BENCHMARK", "OPS/SEC", "LATENCY", "TOTAL OPS", "ERRORS")
	color.Cyan("╠══════════════════════════════════════════════════════════════════════════════╣")

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
		fmt.Println(status("║ %-20s │ %12.0f │ %12s │ %10d │ %6d ║",
			r.Name, r.OpsPerSec, r.Latency.Round(time.Microsecond), r.Ops, r.Errors))
	}
	color.Cyan("╚══════════════════════════════════════════════════════════════════════════════╝")

	if len(results) > 0 {
		best := results[0]
		color.Green("\n🏆 Best: %s - %.0f ops/sec", best.Name, best.OpsPerSec)
	}
}

func printSingleResult(r BenchmarkResult) {
	fmt.Println()
	color.Cyan("╔══════════════════════════════════════════════════════════╗")
	color.Cyan("║                  BENCHMARK RESULT                         ║")
	color.Cyan("╠══════════════════════════════════════════════════════════╣")

	status := "✅ PASS"
	statusColor := color.GreenString
	if r.Errors > 0 {
		status = "❌ FAIL"
		statusColor = color.RedString
	}

	fmt.Printf("║ %-25s %-30s ║\n", "Benchmark:", color.WhiteString(r.Name))
	fmt.Printf("║ %-25s %-30s ║\n", "Status:", statusColor(status))
	fmt.Printf("║ %-25s %-30s ║\n", "Total Operations:", color.YellowString("%d", r.Ops))
	fmt.Printf("║ %-25s %-30s ║\n", "Duration:", color.YellowString("%v", r.Duration.Round(time.Millisecond)))
	fmt.Printf("║ %-25s %-30s ║\n", "Throughput:", color.GreenString("%.0f ops/sec", r.OpsPerSec))
	fmt.Printf("║ %-25s %-30s ║\n", "Avg Latency:", color.YellowString("%v", r.Latency.Round(time.Microsecond)))
	fmt.Printf("║ %-25s %-30s ║\n", "Errors:", color.RedString("%d", r.Errors))
	color.Cyan("╚══════════════════════════════════════════════════════════╝")

	if r.OpsPerSec > 1000000 {
		color.Green("\n🔥 Excellent performance! (> 1M ops/sec)")
	} else if r.OpsPerSec > 100000 {
		color.Yellow("\n👍 Good performance (> 100K ops/sec)")
	} else {
		color.Red("\n⚠️  Performance below expectations (< 100K ops/sec)")
	}
}

func printSystemInfo() {
	fmt.Println()
	color.Cyan("╔══════════════════════════════════════════════════════════╗")
	color.Cyan("║                    SYSTEM INFO                            ║")
	color.Cyan("╠══════════════════════════════════════════════════════════╣")
	fmt.Printf("║ %-25s %-30s ║\n", "OS:", runtime.GOOS)
	fmt.Printf("║ %-25s %-30s ║\n", "Arch:", runtime.GOARCH)
	fmt.Printf("║ %-25s %-30s ║\n", "CPU Cores:", fmt.Sprintf("%d", runtime.NumCPU()))
	fmt.Printf("║ %-25s %-30s ║\n", "Go Version:", runtime.Version())
	color.Cyan("╚══════════════════════════════════════════════════════════╝")
}
