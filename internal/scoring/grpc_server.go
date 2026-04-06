package scoring

import (
	"context"
	"log/slog"
	"net"

	"github.com/AndrewAct/bidflock/gen/go/scoring"
	_ "github.com/AndrewAct/bidflock/pkg/codec"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type GRPCServer struct {
	svc    *Service
	logger *slog.Logger
}

func NewGRPCServer(svc *Service, logger *slog.Logger) *GRPCServer {
	return &GRPCServer{svc: svc, logger: logger}
}

func (g *GRPCServer) ScoreAds(ctx context.Context, req *scoring.ScoreRequest) (*scoring.ScoreResponse, error) {
	return g.svc.ScoreAds(ctx, req)
}

func (g *GRPCServer) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	srv := grpc.NewServer()
	scoring.RegisterScoringServiceServer(srv, g)
	reflection.Register(srv)
	g.logger.Info("scoring gRPC server listening", "addr", addr)
	return srv.Serve(lis)
}
