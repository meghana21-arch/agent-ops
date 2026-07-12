package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/agentops/runtime/internal/agents"
	"github.com/agentops/runtime/internal/db"
	"github.com/agentops/runtime/internal/llm"
	"github.com/agentops/runtime/internal/runs"
	"github.com/agentops/runtime/internal/tools"
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

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		log.Fatal("ANTHROPIC_API_KEY is required")
	}

	workspaceDir := os.Getenv("WORKSPACE_DIR")
	if workspaceDir == "" {
		workspaceDir = "/tmp/agentops-workspace"
	}

	loop := &agentLoop{
		runRepo:   runs.NewRepository(pool),
		agentRepo: agents.NewRepository(pool),
		toolSvc:   tools.NewService(workspaceDir, tools.NewRepository(pool)),
		llm:       llm.NewClient(apiKey),
	}

	log.Println("Worker started — polling for pending runs every 5s")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	ticker := time.NewTicker(5 * time.Second)
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
			pollAndProcess(ctx, loop)
		}
	}
}

// pollAndProcess picks up all CREATED runs and launches each in a goroutine.
func pollAndProcess(ctx context.Context, loop *agentLoop) {
	pending, err := loop.runRepo.ListByStatus(ctx, runs.StatusCreated)
	if err != nil {
		log.Printf("poll error: %v", err)
		return
	}
	for _, run := range pending {
		run := run
		go loop.processRun(ctx, &run)
	}
}
