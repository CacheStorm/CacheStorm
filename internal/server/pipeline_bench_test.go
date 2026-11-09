package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
)

// BenchmarkPipelinedConnection sends pipelines of 16 commands over one TCP
// connection and reads the replies, as redis-benchmark -c 1 -P 16 does: the
// whole per-command path of one connection, parsing to reply, with the
// loopback round trip shared by the pipeline. ns/op is per command.
func BenchmarkPipelinedConnection(b *testing.B) {
	for _, tc := range []struct{ name, cmd string }{
		{"SET", "*3\r\n$3\r\nSET\r\n$7\r\nkey:%03d\r\n$5\r\nvalue\r\n"},
		{"GET", "*2\r\n$3\r\nGET\r\n$7\r\nkey:%03d\r\n"},
	} {
		b.Run(tc.name, func(b *testing.B) {
			cfg := &config.Config{}
			cfg.Server.Bind, cfg.Server.Port = "127.0.0.1", 0
			s, err := New(cfg)
			if err != nil {
				b.Fatal(err)
			}
			if err := s.Start(context.Background()); err != nil {
				b.Fatal(err)
			}
			defer s.Stop(context.Background())
			conn, err := net.DialTimeout("tcp", s.listener.Addr().String(), time.Second)
			if err != nil {
				b.Fatal(err)
			}
			defer conn.Close()

			const depth = 16
			var batch []byte
			for i := range depth {
				batch = fmt.Appendf(batch, tc.cmd, i)
			}
			// Each reply is one line: +OK or $5 then the value.
			lines := depth
			if tc.name == "GET" {
				lines = 2 * depth
				set := []byte{}
				for i := range depth {
					set = fmt.Appendf(set, "*3\r\n$3\r\nSET\r\n$7\r\nkey:%03d\r\n$5\r\nvalue\r\n", i)
				}
				if _, err := conn.Write(set); err != nil {
					b.Fatal(err)
				}
				if err := readLines(bufio.NewReader(conn), depth); err != nil {
					b.Fatal(err)
				}
			}
			rd := bufio.NewReaderSize(conn, 64<<10)
			b.ResetTimer()
			for i := 0; i < b.N; i += depth {
				if _, err := conn.Write(batch); err != nil {
					b.Fatal(err)
				}
				if err := readLines(rd, lines); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func readLines(rd *bufio.Reader, n int) error {
	for ; n > 0; n-- {
		line, err := rd.ReadSlice('\n')
		if err != nil {
			return err
		}
		if line[0] == '-' {
			return fmt.Errorf("error reply %q", line)
		}
	}
	return nil
}
