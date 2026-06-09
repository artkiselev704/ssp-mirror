package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"time"
)

var (
	gConfig Config
)

type Config struct {
	Host     string `json:"host"`
	Target   string `json:"target"`
	Timeout  int    `json:"timeout"`
	LogLevel int    `json:"log_level"`
}

func LoadConfig() error {
	// Read config.json
	file, err := os.Open("config.json")
	if err != nil {
		return err
	}
	defer file.Close()

	return json.NewDecoder(file).Decode(&gConfig)
}

func HandleSession(srcConn net.Conn) {
	// Handle current session
	RemoteAddr := srcConn.RemoteAddr().String()
	slog.Info("New session",
		slog.String("RemoteAddr", RemoteAddr),
		slog.Int("NumGoroutine", runtime.NumGoroutine()),
	)
	defer func() {
		slog.Info("Session finished",
			slog.String("RemoteAddr", RemoteAddr),
			slog.Int("NumGoroutine", runtime.NumGoroutine()),
		)
		srcConn.Close()
	}()

	// Connect to the target
	tgtConn, err := net.DialTimeout("tcp", gConfig.Target, time.Duration(gConfig.Timeout)*time.Second)
	if err != nil {
		slog.Error("Failed to connect to the target",
			slog.String("RemoteAddr", RemoteAddr),
			slog.String("err", err.Error()),
		)
		return
	}
	defer tgtConn.Close()

	// Proxy data
	exitCh := make(chan struct{}, 1)

	go func() { // source -> target
		_, err = io.Copy(tgtConn, srcConn)
		if err != nil {
			slog.Debug("source -> target error", slog.String("err", err.Error()))
		}
		exitCh <- struct{}{}
	}()

	go func() { // target -> source
		_, err = io.Copy(srcConn, tgtConn)
		if err != nil {
			slog.Debug("target -> source error", slog.String("err", err.Error()))
		}
		exitCh <- struct{}{}
	}()

	<-exitCh
}

func main() {
	// Load config
	err := LoadConfig()
	if err != nil {
		slog.Error("failed to load config", slog.String("err", err.Error()))
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(slog.Level(gConfig.LogLevel))

	// Setup listener
	listener, err := net.Listen("tcp", gConfig.Host)
	if err != nil {
		slog.Error("failed to setup listener", slog.String("err", err.Error()))
		os.Exit(1)
	}
	defer listener.Close()

	// Accept connections
	slog.Info("mirror started and ready to accept connections", slog.String("host", listener.Addr().String()))

	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn("failed to accept connection", slog.String("err", err.Error()))
			continue
		}

		go HandleSession(conn)
	}
}
