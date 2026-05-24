package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"os"
	"runtime"
	"sync"
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
	defer func() {
		CloseFile(file)
	}()

	return json.NewDecoder(file).Decode(&gConfig)
}

func HandleSession(srcConn net.Conn) {
	// Handle current session
	slog.Info("new session",
		slog.String("srcAddr", srcConn.RemoteAddr().String()),
	)
	defer func() {
		CloseConnection(srcConn)
		slog.Debug("session closed", slog.Int("goroutine_num", runtime.NumGoroutine()))
	}()

	// Connect to the target
	tgtConn, err := net.DialTimeout("tcp", gConfig.Target, time.Duration(gConfig.Timeout)*time.Second)
	if err != nil {
		slog.Error("failed to connect to the target", slog.String("err", err.Error()))
		return
	}
	defer func() {
		CloseConnection(tgtConn)
	}()

	// Proxy data
	var wg sync.WaitGroup

	wg.Add(2)

	go func() { // from source to target
		_, err = io.Copy(tgtConn, srcConn)
		if err != nil {
			slog.Debug("source to target error", slog.String("err", err.Error()))
		}

		tcpConn, ok := tgtConn.(*net.TCPConn)
		if ok {
			err = tcpConn.CloseWrite()
			if err != nil {
				slog.Debug("target close write error", slog.String("err", err.Error()))
			}
		}

		wg.Done()
	}()

	go func() { // from target to source
		_, err = io.Copy(srcConn, tgtConn)
		if err != nil {
			slog.Debug("target to source error", slog.String("err", err.Error()))
		}

		tcpConn, ok := srcConn.(*net.TCPConn)
		if ok {
			err = tcpConn.CloseWrite()
			if err != nil {
				slog.Debug("source close write error", slog.String("err", err.Error()))
			}
		}

		wg.Done()
	}()

	wg.Wait()
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
	defer func() {
		err = listener.Close()
		if err != nil {
			slog.Warn("failed to close listener", slog.String("err", err.Error()))
		}
	}()

	slog.Info("mirror started and ready to accept connections", slog.String("host", listener.Addr().String()))

	// Wait for connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			slog.Warn("failed to accept connection", slog.String("err", err.Error()))
		} else {
			go HandleSession(conn)
		}
	}
}
