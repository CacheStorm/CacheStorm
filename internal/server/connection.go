package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"runtime/debug"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

const (
	defaultReadTimeout  = 5 * time.Minute
	defaultWriteTimeout = 5 * time.Minute
	keepAlivePeriod     = 3 * time.Minute
)

type Connection struct {
	ID           int64
	conn         net.Conn
	reader       *resp.Reader
	writer       *resp.Writer
	store        *store.Store
	router       *command.Router
	namespace    string
	createdAt    time.Time
	lastCmd      string
	readTimeout  time.Duration
	writeTimeout time.Duration
	subscriber   *store.Subscriber // PubSub subscriber, persists across commands
}

func NewConnection(id int64, conn net.Conn, s *store.Store, r *command.Router) *Connection {
	return &Connection{
		ID:           id,
		conn:         conn,
		reader:       resp.NewReader(bufio.NewReader(conn)),
		writer:       resp.NewWriter(bufio.NewWriter(conn)),
		store:        s,
		router:       r,
		namespace:    "default",
		createdAt:    time.Now(),
		readTimeout:  defaultReadTimeout,
		writeTimeout: defaultWriteTimeout,
	}
}

func (c *Connection) Handle() {
	defer c.Close()
	defer c.recoverPanic()

	logger.Debug().
		Int64("conn_id", c.ID).
		Str("remote", c.conn.RemoteAddr().String()).
		Msg("client connected")

	if tcpConn, ok := c.conn.(*net.TCPConn); ok {
		tcpConn.SetKeepAlive(true)
		tcpConn.SetKeepAlivePeriod(keepAlivePeriod)
	}

	for {
		c.conn.SetReadDeadline(time.Now().Add(c.readTimeout))
		cmd, args, err := c.reader.ReadCommand()
		if err != nil {
			if err == io.EOF || isClosedError(err) {
				return
			}
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				return
			}
			logger.Error().Err(err).Int64("conn_id", c.ID).Msg("read error")
			return
		}

		c.lastCmd = cmd

		ctx := command.NewContextWithClient(cmd, args, c.store, c.writer, c.ID, c.conn.RemoteAddr().String())
		// Share the subscriber across commands so PubSub state persists
		if c.subscriber != nil {
			ctx.Subscriber = c.subscriber
		}

		if cmd == "QUIT" {
			c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
			c.writer.WriteOK()
			return
		}

		c.conn.SetWriteDeadline(time.Now().Add(c.writeTimeout))
		if err := c.router.Execute(ctx); err != nil {
			if err == command.ErrUnknownCommand {
				c.writer.WriteError("ERR unknown command '" + cmd + "'")
			} else {
				c.writer.WriteError(err.Error())
			}
		}
		// Capture subscriber if created during this command (e.g. SUBSCRIBE)
		if ctx.Subscriber != nil && c.subscriber == nil {
			c.subscriber = ctx.Subscriber
		}
	}
}

func (c *Connection) recoverPanic() {
	if r := recover(); r != nil {
		logger.Error().
			Int64("conn_id", c.ID).
			Str("last_cmd", c.lastCmd).
			Str("remote", c.conn.RemoteAddr().String()).
			Str("panic", fmt.Sprintf("%v", r)).
			Str("stack", string(debug.Stack())).
			Msg("panic recovered in connection handler")
	}
}

func (c *Connection) Close() {
	// Clean up PubSub subscriber to prevent resource leak
	if c.subscriber != nil {
		if ps := c.store.GetPubSub(); ps != nil {
			ps.RemoveSubscriber(c.subscriber)
		}
		c.subscriber = nil
	}
	c.conn.Close()
	logger.Debug().
		Int64("conn_id", c.ID).
		Msg("client disconnected")
}

func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "use of closed network connection"
}
