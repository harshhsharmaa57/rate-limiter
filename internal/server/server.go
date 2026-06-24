package server

import (
    "context"

    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    pb "github.com/yourusername/ratelimiter/gen/pb"
    "github.com/yourusername/ratelimiter/internal/limiter"
)

// Server implements the gRPC RateLimiterServiceServer interface.
type Server struct {
    pb.UnimplementedRateLimiterServiceServer
    // ↑ IMPORTANT: Always embed this. It provides default "not implemented"
    // responses for any RPCs you haven't implemented yet, so your code compiles
    // even when your proto has more methods than you've written.
    
    limiter *limiter.Limiter
}

// New creates a new Server.
func New(l *limiter.Limiter) *Server {
    return &Server{limiter: l}
}

// ConsumeLimit checks and records a rate limit request.
// YOUR TASK: implement this.
//
// Steps:
//   1. Validate: if req.Key == "" return nil, status.Error(codes.InvalidArgument, "key is required")
//   2. Call l.limiter.Consume(ctx, req.Key, req.RuleId)
//   3. If error: return nil, status.Errorf(codes.Internal, "consume: %v", err)
//   4. Return &pb.LimitResponse{
//          Allowed:   result.Allowed,
//          Remaining: result.Remaining,
//          Limit:     result.Limit,
//      }, nil
func (s *Server) ConsumeLimit(ctx context.Context, req *pb.LimitRequest) (*pb.LimitResponse, error) {
    // YOUR CODE HERE
}

// CheckLimit checks without recording. For now, same as ConsumeLimit.
// YOUR TASK: implement this. Call s.limiter.Check if you have it, or just call Consume.
func (s *Server) CheckLimit(ctx context.Context, req *pb.LimitRequest) (*pb.LimitResponse, error) {
    // YOUR CODE HERE
}

// WatchQuota is a server-streaming RPC.
// For now, implement a stub that immediately returns.
// You'll implement the real version in Part 9.
func (s *Server) WatchQuota(req *pb.WatchRequest, stream pb.RateLimiterService_WatchQuotaServer) error {
    return nil
}