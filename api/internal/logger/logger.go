package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"cloudpass/internal/config"
	"github.com/rs/zerolog"
)

type atomicLogger struct {
	ptr atomic.Pointer[zerolog.Logger]
}

func (a *atomicLogger) Load() *zerolog.Logger {
	return a.ptr.Load()
}

func (a *atomicLogger) Store(l zerolog.Logger) {
	a.ptr.Store(&l)
}

var (
	API       atomicLogger
	Multipass atomicLogger
	Websocket atomicLogger

	logCfg    config.LoggingConfig
	logMu     sync.Mutex
	logOutput io.Writer
)

func init() {
	noop := zerolog.Nop()
	API.Store(noop)
	Multipass.Store(noop)
	Websocket.Store(noop)
}

func Init(cfg config.LoggingConfig) error {
	logMu.Lock()
	defer logMu.Unlock()

	logCfg = cfg

	w, err := createWriter(cfg)
	if err != nil {
		return err
	}
	logOutput = w

	zerolog.TimeFieldFormat = time.RFC3339
	zerolog.SetGlobalLevel(parseLevel(cfg.Level))

	baseLogger := zerolog.New(logOutput)

	api := baseLogger.With().Str("component", "api").Logger()
	mp := baseLogger.With().Str("component", "multipass").Logger()
	ws := baseLogger.With().Str("component", "websocket").Logger()

	if cfg.Levels.API != "" {
		api = api.Level(parseLevel(cfg.Levels.API))
	}
	if cfg.Levels.Multipass != "" {
		mp = mp.Level(parseLevel(cfg.Levels.Multipass))
	}
	if cfg.Levels.Websocket != "" {
		ws = ws.Level(parseLevel(cfg.Levels.Websocket))
	}

	API.Store(api)
	Multipass.Store(mp)
	Websocket.Store(ws)

	return nil
}

// Reinit reinitializes the logger with new configuration at runtime.
func Reinit(cfg config.LoggingConfig) error {
	logMu.Lock()
	defer logMu.Unlock()

	logCfg = cfg

	w, err := createWriter(cfg)
	if err != nil {
		return err
	}
	logOutput = w

	zerolog.SetGlobalLevel(parseLevel(cfg.Level))

	baseLogger := zerolog.New(logOutput)

	api := baseLogger.With().Str("component", "api").Logger()
	mp := baseLogger.With().Str("component", "multipass").Logger()
	ws := baseLogger.With().Str("component", "websocket").Logger()

	if cfg.Levels.API != "" {
		api = api.Level(parseLevel(cfg.Levels.API))
	}
	if cfg.Levels.Multipass != "" {
		mp = mp.Level(parseLevel(cfg.Levels.Multipass))
	}
	if cfg.Levels.Websocket != "" {
		ws = ws.Level(parseLevel(cfg.Levels.Websocket))
	}

	API.Store(api)
	Multipass.Store(mp)
	Websocket.Store(ws)

	return nil
}

func createWriter(cfg config.LoggingConfig) (io.Writer, error) {
	var output io.Writer

	switch cfg.Output {
	case "file":
		dir := filepath.Dir(cfg.File.Path)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}
		f, err := os.OpenFile(cfg.File.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to open log file: %w", err)
		}
		output = &rotatingWriter{
			file:       f,
			maxSizeMB:  cfg.File.MaxSizeMB,
			maxBackups: cfg.File.MaxBackups,
			maxAgeDays: cfg.File.MaxAgeDays,
			compress:   cfg.File.Compress,
		}
	case "syslog":
		output = &syslogWriter{network: cfg.Syslog.Network, address: cfg.Syslog.Address}
	default:
		output = os.Stdout
	}

	if cfg.Format == "console" {
		output = zerolog.ConsoleWriter{Out: output, TimeFormat: time.RFC3339}
	}

	return output, nil
}

type rotatingWriter struct {
	file       *os.File
	maxSizeMB  int
	maxBackups int
	maxAgeDays int
	compress   bool
	size       int64
}

func (r *rotatingWriter) Write(p []byte) (n int, err error) {
	if r.size+int64(len(p)) > int64(r.maxSizeMB*1024*1024) {
		r.rotate()
	}
	n, err = r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *rotatingWriter) rotate() {
	r.file.Close()
	now := time.Now()
	backupName := filepath.Join(filepath.Dir(r.file.Name()),
		fmt.Sprintf("%s.%s.log", filepath.Base(r.file.Name()), now.Format("20060102150405")))
	if err := os.Rename(r.file.Name(), backupName); err != nil {
		println("WARNING: failed to rotate log file: " + err.Error())
	}

	r.file, _ = os.OpenFile(r.file.Name(), os.O_CREATE|os.O_WRONLY, 0644)
	r.size = 0

	r.cleanOldBackups()
}

func (r *rotatingWriter) cleanOldBackups() {
	if r.maxBackups <= 0 {
		return
	}
	dir := filepath.Dir(r.file.Name())
	pattern := filepath.Base(r.file.Name()) + ".*.log"

	files, _ := filepath.Glob(filepath.Join(dir, pattern))
	if len(files) <= r.maxBackups {
		return
	}

	for _, f := range files[:len(files)-r.maxBackups] {
		os.Remove(f)
	}
}

type syslogWriter struct {
	network string
	address string
}

func (s *syslogWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	default:
		return zerolog.InfoLevel
	}
}
