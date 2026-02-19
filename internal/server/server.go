package server

import (
	"context"
	"net"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/cachestorm/cachestorm/internal/command"
	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/store"
)

type Server struct {
	cfg      *config.Config
	listener net.Listener
	router   *command.Router
	store    *store.Store
	conns    sync.Map
	connID   atomic.Int64
	stopping atomic.Bool
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

func New(cfg *config.Config) (*Server, error) {
	s := &Server{
		cfg:    cfg,
		store:  store.NewStore(),
		router: command.NewRouter(),
		stopCh: make(chan struct{}),
	}

	command.RegisterStringCommands(s.router)
	command.RegisterServerCommands(s.router)
	command.RegisterKeyCommands(s.router)
	command.RegisterHashCommands(s.router)
	command.RegisterListCommands(s.router)
	command.RegisterSetCommands(s.router)
	command.RegisterTagCommands(s.router)
	command.RegisterNamespaceCommands(s.router)
	command.RegisterClusterCommands(s.router)
	command.RegisterClientCommands(s.router)
	command.RegisterConfigCommands(s.router)
	command.RegisterTransactionCommands(s.router)
	command.RegisterSortedSetCommands(s.router)
	command.RegisterPubSubCommands(s.router)
	command.RegisterBitmapCommands(s.router)
	command.RegisterHyperLogLogCommands(s.router)
	command.RegisterStreamCommands(s.router)
	command.RegisterGeoCommands(s.router)
	command.RegisterScriptCommands(s.router)

	return s, nil
}

func (s *Server) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.cfg.Server.Bind, strconv.Itoa(s.cfg.Server.Port))
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener

	logger.Info().
		Str("addr", addr).
		Msg("CacheStorm server started")

	go s.acceptLoop()

	return nil
}

func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if s.stopping.Load() {
				return
			}
			logger.Error().Err(err).Msg("accept error")
			continue
		}

		connID := s.connID.Add(1)
		c := NewConnection(connID, conn, s.store, s.router)

		s.conns.Store(connID, c)
		s.wg.Add(1)

		go func() {
			defer s.wg.Done()
			defer s.conns.Delete(connID)
			c.Handle()
		}()
	}
}

func (s *Server) Stop(ctx context.Context) error {
	s.stopping.Store(true)
	close(s.stopCh)

	if s.listener != nil {
		s.listener.Close()
	}

	s.conns.Range(func(key, value interface{}) bool {
		if c, ok := value.(*Connection); ok {
			c.Close()
		}
		return true
	})

	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-ctx.Done():
	}

	logger.Info().Msg("CacheStorm server stopped")
	return nil
}

func (s *Server) Store() *store.Store {
	return s.store
}
