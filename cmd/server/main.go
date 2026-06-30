package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/harshhsharmaa57/rate-limiter.git/gen/pb"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/limiter"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/server"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/store"
	_ "github.com/jackc/pgx/v5/stdlib"
	"google.golang.org/grpc"
)

func main() {
	// Connect to PostgreSQL
	db, err := store.OpenDB("postgres://rl:rl@localhost:5432/ratelimiter?sslmode=disable")
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	// Create rule cache and start loading rules
	ctx := context.Background()
	ruleCache := store.NewRuleCache(db)
	ruleCache.StartRefreshLoop(ctx, 30*time.Second)

	// Create Redis store
	s := store.NewRedisStore("localhost:6379")

	// Create the limiter with Redis store and rule cache
	l := limiter.NewWithCache(s, ruleCache)

	// Create gRPC server implementation
	srv := server.New(l)

	// Start HTTP server for SSE dashboard (in background)
	go func() {
		http.HandleFunc("/quota", func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "key required", 400)
				return
			}

			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("Access-Control-Allow-Origin", "*")

			ch := l.Subscribe(key)
			defer l.Unsubscribe(key, ch)

			flusher := w.(http.Flusher)

			for {
				select {
				case event := <-ch:
					data, _ := json.Marshal(event)
					fmt.Fprintf(w, "data: %s\n\n", data)
					flusher.Flush()
				case <-r.Context().Done():
					return
				}
			}
		})

		http.HandleFunc("/fire", func(w http.ResponseWriter, r *http.Request) {
			key := r.URL.Query().Get("key")
			ruleID := r.URL.Query().Get("rule_id")
			if ruleID == "" {
				ruleID = "free-tier"
			}

			w.Header().Set("Access-Control-Allow-Origin", "*")

			result, err := l.Consume(r.Context(), key, ruleID)
			if err != nil {
				http.Error(w, err.Error(), 500)
				return
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(result)
		})

		log.Println("HTTP server listening on :8080")
		http.ListenAndServe(":8080", nil)
	}()

	// Listen on port 50051
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// Create gRPC server
	grpcServer := grpc.NewServer()

	// Register implementation
	pb.RegisterRateLimiterServiceServer(grpcServer, srv)

	log.Println("rate limiter listening on :50051")

	// Start serving
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
