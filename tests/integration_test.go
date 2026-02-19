package main

import (
	"bufio"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestIntegrationPing(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	resp := "*1\r\n$4\r\nPING\r\n"
	conn.Write([]byte(resp))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != "+PONG\r\n" {
		t.Errorf("expected +PONG, got %s", response)
	}
}

func TestIntegrationSetGet(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	setCmd := "*3\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"
	conn.Write([]byte(setCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != "+OK\r\n" {
		t.Errorf("expected +OK, got %s", response)
	}

	getCmd := "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"
	conn.Write([]byte(getCmd))

	line, _ := reader.ReadString('\n')
	if line != "$3\r\n" {
		t.Errorf("expected $3, got %s", line)
	}

	data, _ := reader.ReadString('\n')
	if data != "bar\r\n" {
		t.Errorf("expected bar, got %s", data)
	}
}

func TestIntegrationIncr(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	setCmd := "*3\r\n$3\r\nSET\r$$6\r\ncounter\r\n$1\r\n0\r\n"
	conn.Write([]byte(setCmd))

	reader := bufio.NewReader(conn)
	reader.ReadString('\n')

	incrCmd := "*2\r\n$4\r\nINCR\r\n$7\r\ncounter\r\n"
	conn.Write([]byte(incrCmd))

	response, _ := reader.ReadString('\n')
	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}
}

func TestIntegrationHash(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	hsetCmd := "*4\r\n$4\r\nHSET\r\n$6\r\nmyhash\r\n$5\r\nfield\r\n$5\r\nvalue\r\n"
	conn.Write([]byte(hsetCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}

	hgetCmd := "*3\r\n$4\r\nHGET\r\n$6\r\nmyhash\r\n$5\r\nfield\r\n"
	conn.Write([]byte(hgetCmd))

	line, _ := reader.ReadString('\n')
	if line != "$5\r\n" {
		t.Errorf("expected $5, got %s", line)
	}

	data, _ := reader.ReadString('\n')
	if data != "value\r\n" {
		t.Errorf("expected value, got %s", data)
	}
}

func TestIntegrationList(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	lpushCmd := "*3\r\n$5\r\nLPUSH\r\n$6\r\nmylist\r\n$5\r\nhello\r\n"
	conn.Write([]byte(lpushCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}

	rpopCmd := "*2\r\n$4\r\nRPOP\r\n$6\r\nmylist\r\n"
	conn.Write([]byte(rpopCmd))

	line, _ := reader.ReadString('\n')
	if line != "$5\r\n" {
		t.Errorf("expected $5, got %s", line)
	}

	data, _ := reader.ReadString('\n')
	if data != "hello\r\n" {
		t.Errorf("expected hello, got %s", data)
	}
}

func TestIntegrationSet(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	saddCmd := "*3\r\n$4\r\nSADD\r\n$6\r\nmyset\r\n$6\r\nmember\r\n"
	conn.Write([]byte(saddCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}

	sismemberCmd := "*3\r\n$9\r\nSISMEMBER\r\n$6\r\nmyset\r\n$6\r\nmember\r\n"
	conn.Write([]byte(sismemberCmd))

	response, _ = reader.ReadString('\n')
	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}
}

func TestIntegrationTTL(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	setCmd := "*4\r\n$3\r\nSET\r\n$3\r\nfoo\r\n$3\r\nbar\r\n$2\r\nEX\r\n$2\r\n10\r\n"
	conn.Write([]byte(setCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != "+OK\r\n" {
		t.Errorf("expected +OK, got %s", response)
	}

	ttlCmd := "*2\r\n$3\r\nTTL\r\n$3\r\nfoo\r\n"
	conn.Write([]byte(ttlCmd))

	response, _ = reader.ReadString('\n')
	if response != ":10\r\n" && response != ":9\r\n" {
		t.Errorf("expected :10 or :9, got %s", response)
	}
}

func TestIntegrationTagInvalidation(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	settagCmd := "*4\r\n$6\r\nSETTAG\r\n$6\r\nuser:1\r\n$4\r\nJohn\r\n$5\r\nusers\r\n"
	conn.Write([]byte(settagCmd))

	reader := bufio.NewReader(conn)
	response, _ := reader.ReadString('\n')

	if response != "+OK\r\n" {
		t.Errorf("expected +OK, got %s", response)
	}

	tagkeysCmd := "*2\r\n$7\r\nTAGKEYS\r\n$5\r\nusers\r\n"
	conn.Write([]byte(tagkeysCmd))

	response, _ = reader.ReadString('\n')
	if response != "*1\r\n" {
		t.Errorf("expected *1, got %s", response)
	}

	invalidateCmd := "*2\r\n$10\r\nINVALIDATE\r\n$5\r\nusers\r\n"
	conn.Write([]byte(invalidateCmd))

	response, _ = reader.ReadString('\n')
	if response != ":1\r\n" {
		t.Errorf("expected :1, got %s", response)
	}

	getCmd := "*2\r\n$3\r\nGET\r\n$6\r\nuser:1\r\n"
	conn.Write([]byte(getCmd))

	response, _ = reader.ReadString('\n')
	if response != "$-1\r\n" {
		t.Errorf("expected $-1 (null), got %s", response)
	}
}

func TestIntegrationInfo(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	infoCmd := "*1\r\n$4\r\nINFO\r\n"
	conn.Write([]byte(infoCmd))

	reader := bufio.NewReader(conn)
	line, _ := reader.ReadString('\n')

	if line[0] != '$' {
		t.Errorf("expected bulk string response, got %s", line)
	}
}

func TestIntegrationConcurrent(t *testing.T) {
	const numClients = 10
	const numOps = 100

	errCh := make(chan error, numClients)

	for i := 0; i < numClients; i++ {
		go func(id int) {
			conn, err := net.Dial("tcp", "localhost:6380")
			if err != nil {
				errCh <- err
				return
			}
			defer conn.Close()

			reader := bufio.NewReader(conn)
			for j := 0; j < numOps; j++ {
				key := fmt.Sprintf("key:%d:%d", id, j)
				setCmd := fmt.Sprintf("*3\r\n$3\r\nSET\r\n$%d\r\n%s\r\n$5\r\nvalue\r\n", len(key), key)
				conn.Write([]byte(setCmd))
				reader.ReadString('\n')
			}
			errCh <- nil
		}(i)
	}

	for i := 0; i < numClients; i++ {
		if err := <-errCh; err != nil {
			t.Errorf("client error: %v", err)
		}
	}
}

func TestIntegrationPipeline(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:6380")
	if err != nil {
		t.Skipf("server not running: %v", err)
	}
	defer conn.Close()

	set1 := "*3\r\n$3\r\nSET\r\n$3\r\nk1\r\n$2\r\nv1\r\n"
	set2 := "*3\r\n$3\r\nSET\r\n$3\r\nk2\r\n$2\r\nv2\r\n"
	get1 := "*2\r\n$3\r\nGET\r\n$2\r\nk1\r\n"
	get2 := "*2\r\n$3\r\nGET\r\n$2\r\nk2\r\n"

	conn.Write([]byte(set1 + set2 + get1 + get2))

	reader := bufio.NewReader(conn)

	for i := 0; i < 4; i++ {
		_, err := reader.ReadString('\n')
		if err != nil {
			t.Errorf("error reading response %d: %v", i, err)
		}
	}
}

func init() {
	time.Sleep(100 * time.Millisecond)
}
