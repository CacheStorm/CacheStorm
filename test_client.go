package main

import (
	"bufio"
	"fmt"
	"net"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		fmt.Printf("Connection failed: %v\n", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to CacheStorm!")

	resp := "*1\r\n$4\r\nPING\r\n"
	conn.Write([]byte(resp))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')
	fmt.Printf("PING Response: %s", response)

	setCmd := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	conn.Write([]byte(setCmd))
	response, _ = reader.ReadString('\n')
	fmt.Printf("SET Response: %s", response)

	getCmd := "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"
	conn.Write([]byte(getCmd))

	line, _ := reader.ReadString('\n')
	fmt.Printf("GET Response (type): %s", line)
	line, _ = reader.ReadString('\n')
	fmt.Printf("GET Response (data): %s", line)
	line, _ = reader.ReadString('\n')
	fmt.Printf("GET Response (end): %s", line)
}
