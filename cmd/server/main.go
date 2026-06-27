package main

import (
	"log"
	"net"

	"github.com/harshhsharmaa57/rate-limiter.git/gen/pb"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/limiter"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/server"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	// 1. Create the store (in-memory for now)
	s := store.NewRedisStore("localhost:6379")

	// 2. Create the limiter with that store
	l := limiter.New(s)

	// 3. Add some rules so the server has something to use
	l.AddRule(limiter.Rule{
		ID:         "free-tier",
		LimitCount: 10,
		WindowMs:   60,
	})

	l.AddRule(limiter.Rule{
		ID:         "pro-tier",
		LimitCount: 1000,
		WindowMs:   60,
	})

	// 4. Create your gRPC server implementation
	srv := server.New(l)

	// 5. Listen on port 50051 (conventional gRPC port)
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	// 6. Create a gRPC server
	grpcServer := grpc.NewServer()

	// 7. Register your implementation
	pb.RegisterRateLimiterServiceServer(grpcServer, srv)
	reflection.Register(grpcServer)

	log.Println("rate limiter listening on :50051")

	// 8. Start serving (this blocks forever)
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
