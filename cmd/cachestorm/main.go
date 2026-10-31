package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cachestorm/cachestorm/internal/config"
	"github.com/cachestorm/cachestorm/internal/logger"
	"github.com/cachestorm/cachestorm/internal/server"
	"github.com/cachestorm/cachestorm/internal/version"
)

var (
	configPath  = flag.String("config", "", "path to config file (YAML, or a redis.conf when it ends in .conf)")
	bind        = flag.String("bind", "", "bind address")
	port        = flag.Int("port", 0, "server port")
	showVersion = flag.Bool("version", false, "print version and exit")
)

// goFlags are the options the flag package parses; any other "--name"
// option is a Redis configuration directive.
var goFlags = map[string]bool{"config": true, "bind": true, "port": true, "version": true, "h": true, "help": true}

// splitArgs separates the flag package's options from Redis-style ones, as
// redis-server takes them: "cachestorm [config-file] [--option value ...]".
func splitArgs(args []string) (flags []string, file string, redis []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		file, args = args[0], args[1:]
	}
	for i := 0; i < len(args); i++ {
		a := args[i]
		name := strings.TrimLeft(a, "-")
		if j := strings.IndexByte(name, '='); j >= 0 {
			name = name[:j]
		}
		if strings.HasPrefix(a, "-") && goFlags[name] {
			flags = append(flags, a)
			if !strings.Contains(a, "=") && name != "version" && name != "h" && name != "help" && i+1 < len(args) {
				i++
				flags = append(flags, args[i])
			}
			continue
		}
		redis = append(redis, args[i:]...)
		break
	}
	return flags, file, redis
}

func main() {
	flagArgs, file, redisArgs := splitArgs(os.Args[1:])
	if err := flag.CommandLine.Parse(flagArgs); err != nil {
		os.Exit(2)
	}

	if *showVersion {
		fmt.Printf("cachestorm %s (built %s)\n", version.Version, version.BuildTime)
		return
	}
	if file != "" && *configPath == "" {
		*configPath = file
	}

	cfg, rest, err := loadConfig(*configPath, redisArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if *bind != "" {
		cfg.Server.Bind = *bind
	}
	if *port != 0 {
		cfg.Server.Port = *port
	}

	logger.Init(cfg.Logging.Level, cfg.Logging.Format, cfg.Logging.Output)

	server.ManageGoMemoryLimit.Store(true)
	server.ApplyDefaultGCPercent()
	srv, err := server.New(cfg)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create server")
	}
	if err := srv.ApplyConfigDirectives(rest); err != nil {
		logger.Fatal().Err(err).Msg("invalid configuration directive")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := srv.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("failed to start server")
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		logger.Info().Msg("shutdown signal received")
	case <-srv.ShutdownRequested():
	}
	signal.Stop(sigCh)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := srv.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("shutdown error")
		//nolint:gocritic // process is terminating; srv.Stop already performed all cleanup
		os.Exit(1)
	}
}

// loadConfig reads the configuration: a YAML file, or a redis.conf when the
// path ends in .conf, then the Redis-style command-line options on top, as
// redis-server lets them override its file. It returns the directives left
// for CONFIG SET once the server exists.
func loadConfig(path string, redisArgs []string) (*config.Config, []config.Directive, error) {
	var directives []config.Directive
	if strings.HasSuffix(path, ".conf") {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, nil, err
		}
		if directives, err = config.ParseRedisConf(string(data), path); err != nil {
			return nil, nil, err
		}
		path = ""
	}
	args, err := config.ParseRedisArgs(redisArgs)
	if err != nil {
		return nil, nil, err
	}
	directives = append(directives, args...)
	cfg, err := config.Load(path)
	if err != nil {
		return nil, nil, err
	}
	if len(directives) == 0 {
		return cfg, nil, nil
	}
	rest, err := cfg.ApplyDirectives(directives)
	if err != nil {
		return nil, nil, err
	}
	if err := config.Validate(cfg); err != nil {
		return nil, nil, err
	}
	return cfg, rest, nil
}
