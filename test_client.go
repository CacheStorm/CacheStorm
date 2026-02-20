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
	if _, err := conn.Write([]byte(resp)); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}
	fmt.Printf("PING Response: %s", response)

	setCmd := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	if _, err := conn.Write([]byte(setCmd)); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}
	response, err = reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}
	fmt.Printf("SET Response: %s", response)

	getCmd := "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"
	if _, err := conn.Write([]byte(getCmd)); err != nil {
		fmt.Printf("Write failed: %v\n", err)
		return
	}

	line, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}
	fmt.Printf("GET Response (type): %s", line)
	line, err = reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}
	fmt.Printf("GET Response (data): %s", line)
	line, err = reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Read failed: %v\n", err)
		return
	}
	fmt.Printf("GET Response (end): %s", line)
}
