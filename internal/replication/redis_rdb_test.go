package replication

import (
	"bufio"
	"strings"
	"testing"
)

func TestReadUntilMarkReadsADisklessTransfer(t *testing.T) {
	mark := strings.Repeat("m", 39) + "k"
	br := bufio.NewReader(strings.NewReader("REDIS0011 body with m and k" + mark + "*1\r\n$4\r\nPING\r\n"))
	data, err := readUntilMark(br, mark)
	if err != nil || string(data) != "REDIS0011 body with m and k" {
		t.Fatalf("readUntilMark = %q, %v", data, err)
	}
	if rest, _ := br.ReadString('\n'); rest != "*1\r\n" {
		t.Fatalf("stream after the mark starts %q", rest)
	}
	if _, err := readUntilMark(bufio.NewReader(strings.NewReader("x")), "short"); err == nil {
		t.Fatal("a mark that is not 40 bytes was accepted")
	}
}
