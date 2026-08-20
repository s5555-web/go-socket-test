package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"sket/internal/api"
	"sket/internal/api/socket"
	"sket/internal/config"
	"sket/internal/store"
)

func main() {
	os.Exit(run())
}

func run() int {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "panic: %v\n", r)
			os.Exit(1)
		}
	}()

	configPath := "configs/config.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		return 1
	}

	hub := socket.NewHub()
	go hub.Run()
	defer hub.Stop()

	db, err := store.Open(cfg.Database.DSN)
	if err != nil {
		fmt.Fprintf(os.Stderr, "database: %v\n", err)
		return 1
	}
	defer db.Close()
	app := api.New(cfg, db, hub)
	clientSrv := &http.Server{Addr: cfg.Server.ClientAddr, Handler: app.ClientEngine(cfg)}
	adminSrv := &http.Server{Addr: cfg.Server.AdminAddr, Handler: app.AdminEngine()}

	go func() {
		fmt.Printf("client listening on %s\n", clientSrv.Addr)
		if err := clientSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		}
	}()
	go func() {
		fmt.Printf("admin listening on %s\n", adminSrv.Addr)
		if err := adminSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "admin listen: %v\n", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := clientSrv.Shutdown(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "shutdown: %v\n", err)
	}
	_ = adminSrv.Shutdown(ctx)
	fmt.Println("server stopped")
	return 0
}
