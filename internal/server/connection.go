package server

import (
	"bufio"
	"net"
	"time"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
	"github.com/cachestorm/cachestorm/internal/store"
)

type Connection struct {
	ID        int64
	conn      net.Conn
	reader    *resp.Reader
	writer    *resp.Writer
	store     *store.Store
	router    *command.Router
	namespace string
	createdAt time.Time
	lastCmd   string
}

func NewConnection(id int64, conn net.Conn, s *store.Store, r *command.Router) *Connection {
	return &Connection{
		ID:        id,
		conn:      conn,
		reader:    resp.NewReader(bufio.NewReader(conn)),
		writer:    resp.NewWriter(bufio.NewWriter(conn)),
		store:     s,
		router:    r,
		namespace: "default",
		createdAt: time.Now(),
	}
}

func (c *Connection) Handle() {
	defer c.Close()

	logger.Debug().
		Int64("conn_id", c.ID).
		Str("remote", c.conn.RemoteAddr().String()).
		Msg("client connected")

	for {
		cmd, args, err := c.reader.ReadCommand()
		if err != nil {
			if err.Error() == "EOF" {
				return
			}
			logger.Error().Err(err).Int64("conn_id", c.ID).Msg("read error")
			return
		}

		c.lastCmd = cmd

		ctx := command.NewContextWithClient(cmd, args, c.store, c.writer, c.ID, c.conn.RemoteAddr().String())

		if cmd == "QUIT" {
			c.writer.WriteOK()
			return
		}

		if err := c.router.Execute(ctx); err != nil {
			if err == command.ErrUnknownCommand {
				c.writer.WriteError("ERR unknown command '" + cmd + "'")
			} else {
				c.writer.WriteError(err.Error())
			}
		}
	}
}

func (c *Connection) Close() {
	c.conn.Close()
	logger.Debug().
		Int64("conn_id", c.ID).
		Msg("client disconnected")
}

func (c *Connection) RemoteAddr() string {
	return c.conn.RemoteAddr().String()
}
