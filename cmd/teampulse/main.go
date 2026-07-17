package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/italic-jinxin/team-pulse/internal/app"
)

func main() {
	host := flag.String("host", "127.0.0.1", "listen host")
	port := flag.Int("port", 19421, "preferred listen port")
	dataDir := flag.String("data-dir", "", "application data directory")
	flag.Parse()
	if err := validateHost(*host); err != nil {
		slog.Error("listen host", "error", err)
		os.Exit(2)
	}

	a, err := app.New(*dataDir)
	if err != nil {
		slog.Error("initialize", "error", err)
		os.Exit(1)
	}
	defer a.Close()

	listener, actualPort, err := listen(*host, *port)
	if err != nil {
		slog.Error("listen", "error", err)
		os.Exit(1)
	}
	url := fmt.Sprintf("http://%s:%d", *host, actualPort)
	if err := a.WriteRuntimeState(os.Getpid(), *host, actualPort, url); err != nil {
		slog.Warn("write state", "error", err)
	}
	defer a.RemoveRuntimeState()

	server := &http.Server{Handler: a.Router(url), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := server.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("serve", "error", err)
		}
	}()
	fmt.Printf("{\"event\":\"server_ready\",\"url\":%q}\n", url)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	select {
	case <-stop:
	case <-a.ShutdownRequested():
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = server.Shutdown(ctx)
}

func validateHost(host string) error {
	if host != "127.0.0.1" {
		return fmt.Errorf("TeamPulse only listens on 127.0.0.1")
	}
	return nil
}

func listen(host string, preferred int) (net.Listener, int, error) {
	for p := preferred; p <= preferred+100; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", host, p))
		if err == nil {
			return l, p, nil
		}
	}
	return nil, 0, fmt.Errorf("no free port in %d-%d", preferred, preferred+100)
}
