package main_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/harshhsharmaa57/rate-limiter.git/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// The server must already be running: go run cmd/server/main.go
func BenchmarkConsumeLimit(b *testing.B) {
	conn, err := grpc.Dial("localhost:50051",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		b.Fatalf("connect failed: %v", err)
	}
	defer conn.Close()

	client := pb.NewRateLimiterServiceClient(conn)
	ctx := context.Background()

	b.ResetTimer()

	b.RunParallel(func(p *testing.PB) {
		i := 0
		for p.Next() {
			key := fmt.Sprintf("bench-key-%d", i%500)
			_, _ = client.ConsumeLimit(ctx, &pb.LimitRequest{
				Key:    key,
				RuleId: "pro-tier",
			})
			i++
		}
	})
}
