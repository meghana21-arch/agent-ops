package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"

	"github.com/agentops/runtime/internal/agents"
	"github.com/agentops/runtime/internal/db"
	"github.com/agentops/runtime/internal/middleware"
	"github.com/agentops/runtime/internal/projects"
	"github.com/agentops/runtime/internal/runs"
)

func main() {
	ctx := context.Background()

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
	if err := redisClient.Ping(ctx).Err(); err != nil {
		log.Fatalf("connect to redis: %v", err)
	}
	defer redisClient.Close()

	log.Println("Connected to PostgreSQL and Redis")

	r := gin.Default()
	r.Use(middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "0.1.0"})
	})

	v1 := r.Group("/v1")

	projectRepo := projects.NewRepository(pool)
	projectSvc := projects.NewService(projectRepo)
	projects.NewHandler(projectSvc).Register(v1)

	runRepo := runs.NewRepository(pool)
	runSvc := runs.NewService(runRepo)
	runs.NewHandler(runSvc).Register(v1)

	agentRepo := agents.NewRepository(pool)
	agentSvc := agents.NewService(agentRepo)
	agents.NewHandler(agentSvc).Register(v1)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("API server listening on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("start server: %v", err)
	}
}
