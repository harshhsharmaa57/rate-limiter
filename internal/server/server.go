package server

import (
	"context"

	"github.com/harshhsharmaa57/rate-limiter.git/gen/pb"
	"github.com/harshhsharmaa57/rate-limiter.git/internal/limiter"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Server implements the gRPC RateLimiterServiceServer interface.
type Server struct {
	pb.UnimplementedRateLimiterServiceServer
	limiter *limiter.Limiter
}

// New creates a new Server.
func New(l *limiter.Limiter) *Server {
	return &Server{limiter: l}
}

// ConsumeLimit checks and records a rate limit request.
func (s *Server) ConsumeLimit(ctx context.Context, req *pb.LimitRequest) (*pb.LimitResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	result, err := s.limiter.Consume(ctx, req.Key, req.RuleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "consume: %v", err)
	}

	return &pb.LimitResponse{
		Allowed:     result.Allowed,
		Remaining:   result.Remaining,
		Limit:       result.Limit,
		ResetAtUnix: result.ResetAt.Unix(),
	}, nil
}

// CheckLimit checks without recording.
func (s *Server) CheckLimit(ctx context.Context, req *pb.LimitRequest) (*pb.LimitResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	result, err := s.limiter.Check(ctx, req.Key, req.RuleId)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "check: %v", err)
	}

	return &pb.LimitResponse{
		Allowed:     result.Allowed,
		Remaining:   result.Remaining,
		Limit:       result.Limit,
		ResetAtUnix: result.ResetAt.Unix(),
	}, nil
}

// WatchQuota is a server-streaming RPC.
func (s *Server) WatchQuota(req *pb.WatchRequest, stream pb.RateLimiterService_WatchQuotaServer) error {
	if req.Key == "" {
		return status.Error(codes.InvalidArgument, "key is required")
	}

	ctx := stream.Context()

	ch := s.limiter.Subscribe(req.Key)
	defer s.limiter.Unsubscribe(req.Key, ch)

	for {
		select {
		case event := <-ch:
			err := stream.Send(&pb.QuotaUpdate{
				Key:       event.Key,
				Used:      event.Used,
				Remaining: event.Remaining,
				Exceeded:  event.Exceeded,
			})
			if err != nil {
				return err
			}

		case <-ctx.Done():
			return nil
		}
	}
}
