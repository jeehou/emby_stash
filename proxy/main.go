package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--healthcheck" {
		addr := os.Getenv("LISTEN_ADDR")
		if addr == "" {
			addr = "127.0.0.1:18080"
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			os.Exit(1)
		}
		if host == "" || host == "0.0.0.0" || host == "::" {
			host = "127.0.0.1"
		}
		c := http.Client{Timeout: 3 * time.Second}
		r, err := c.Get("http://" + net.JoinHostPort(host, port) + "/healthz")
		if err != nil {
			os.Exit(1)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			os.Exit(1)
		}
		return
	}
	c, err := loadConfig()
	if err != nil {
		log.Fatal(err)
	}
	s := &http.Server{Addr: c.Listen, Handler: newProxy(c), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 64 << 10}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = s.Shutdown(ctx)
	}()
	log.Printf("Stash poster proxy listening on %s; auto face crop, algorithm %s", c.Listen, algorithmVersion)
	if err = s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
