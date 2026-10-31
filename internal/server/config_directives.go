package server

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/resp"
)

// ApplyConfigDirectives applies the Redis configuration directives that
// config.ApplyDirectives left for run time: a "user" line becomes ACL
// SETUSER, anything else CONFIG SET, so they take the same validation as at
// run time.
func (s *Server) ApplyConfigDirectives(ds []config.Directive) error {
	for _, d := range ds {
		var args [][]byte
		cmd := "CONFIG"
		if d.Name == "user" {
			cmd = "ACL"
			args = append(args, []byte("SETUSER"))
			for _, a := range d.Args {
				args = append(args, []byte(a))
			}
		} else {
			if len(d.Args) == 0 {
				return fmt.Errorf("%s: %s: wrong number of arguments", d.Source, d.Name)
			}
			args = append(args, []byte("SET"), []byte(d.Name), []byte(strings.Join(d.Args, " ")))
		}
		reply, err := s.runInternal(cmd, args)
		if err == nil && strings.HasPrefix(reply, "-") {
			err = fmt.Errorf("%s", strings.TrimSpace(reply[1:]))
		}
		if err != nil && cmd == "CONFIG" && isStartupOnly(err) {
			// A parameter Redis reads only at startup and CacheStorm does not
			// use, such as tcp-backlog or pidfile: fine at its current value,
			// ignored with a warning otherwise.
			if current, gerr := s.runInternal("CONFIG", [][]byte{[]byte("GET"), []byte(d.Name)}); gerr != nil || !strings.HasSuffix(strings.TrimSpace(current), "\r\n"+strings.Join(d.Args, " ")) {
				logger.Warn().Str("directive", d.Name).Str("source", d.Source).Msg("CacheStorm does not use this directive; ignored")
			}
			err = nil
		}
		if err != nil {
			return fmt.Errorf("%s: %s: %w", d.Source, d.Name, err)
		}
	}
	return nil
}

func isStartupOnly(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "can't set immutable config") || strings.Contains(msg, "can't set protected config")
}

// runInternal runs one command as an authenticated internal client and
// returns its raw reply.
func (s *Server) runInternal(cmd string, args [][]byte) (string, error) {
	var out bytes.Buffer
	ctx := command.NewContext(cmd, args, s.store, resp.NewWriter(&out))
	ctx.SetAuthenticated(true)
	err := s.router.Execute(ctx)
	if ferr := ctx.Writer.Flush(); err == nil {
		err = ferr
	}
	return out.String(), err
}
