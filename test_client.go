package main

import (
	"context"
	"fmt"
	"log"

	"github.com/harshhsharmaa57/rate-limiter.git/gen/pb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	client := pb.NewRateLimiterServiceClient(conn)
	ctx := context.Background()

	for i := 1; i <= 11; i++ {
		resp, err := client.ConsumeLimit(ctx, &pb.LimitRequest{
			Key:    "user:harsh123",
			RuleId: "free-tier",
		})
		if err != nil {
			log.Printf("req %d: error: %v", i, err)
			continue
		}
		fmt.Printf("req %d: allowed=%v remaining=%d limit=%d\n", i, resp.Allowed, resp.Remaining, resp.Limit)
	}
}
