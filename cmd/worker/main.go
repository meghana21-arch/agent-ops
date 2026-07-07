package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/agentops/runtime/internal/db"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://agentops:agentops@localhost:5432/agentops?sslmode=disable"
	}
	pool, err := db.New(ctx, dbURL)
	if err != nil {
		log.Fatalf("connect to database: %v", err)
	}
	defer pool.Close()

	redisURL := os.Getenv("REDIS_URL")
	if redisURL == "" {
		redisURL = "redis://localhost:6379"
	}
	redisOpts, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatalf("parse redis URL: %v", err)
	}
	redisClient := redis.NewClient(redisOpts)
	defer redisClient.Close()

	log.Println("Worker started — polling for pending runs...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-sigCh:
			log.Println("Worker shutting down gracefully")
			return
		case <-ticker.C:
			if err := redisClient.Set(ctx, "worker:heartbeat:main", time.Now().Unix(), 60*time.Second).Err(); err != nil {
				log.Printf("heartbeat error: %v", err)
			}
			log.Println("Worker heartbeat — Phase 3 will add agent loop here")
		}
	}
}
