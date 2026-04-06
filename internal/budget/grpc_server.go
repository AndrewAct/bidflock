package budget

import (
	"context"
	"log/slog"
	"net"

	"github.com/AndrewAct/bidflock/gen/go/budget"
	_ "github.com/AndrewAct/bidflock/pkg/codec" // register JSON gRPC codec
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

func (g *GRPCServer) CheckBudget(ctx context.Context, req *budget.CheckBudgetRequest) (*budget.CheckBudgetResponse, error) {
	allowed, reason, remaining, pacingMult := g.svc.CheckBudget(ctx, req.CampaignID, req.UserID, req.BidAmount)
	return &budget.CheckBudgetResponse{
		Allowed:              allowed,
		Reason:               reason,
		RemainingDailyBudget: remaining,
		PacingMultiplier:     pacingMult,
	}, nil
}

func (g *GRPCServer) DeductBudget(ctx context.Context, req *budget.DeductBudgetRequest) (*budget.DeductBudgetResponse, error) {
	success, remaining := g.svc.DeductBudget(ctx, req.CampaignID, req.BidID, req.Amount)
	return &budget.DeductBudgetResponse{
		Success:              success,
		RemainingDailyBudget: remaining,
	}, nil
}

func (g *GRPCServer) GetPacingInfo(ctx context.Context, req *budget.PacingRequest) (*budget.PacingResponse, error) {
	// Get remaining budget first
	budgetKey := "budget:daily:" + req.CampaignID
	remaining, _ := g.svc.redis.Raw().Get(ctx, budgetKey).Float64()
	info := g.svc.pacing.GetPacingInfo(ctx, req.CampaignID, remaining)
	if info == nil {
		return &budget.PacingResponse{PacingMultiplier: 1.0}, nil
	}
	return &budget.PacingResponse{
		SpendRate:        info.SpendRate,
		TargetRate:       info.TargetRate,
		PacingMultiplier: info.PacingMultiplier,
		DailySpendSoFar:  info.DailySpendSoFar,
		DailyBudget:      info.DailyBudget,
	}, nil
}

// Serve starts the gRPC server on the given address.
func (g *GRPCServer) Serve(addr string) error {
	lis, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}

	srv := grpc.NewServer()
	budget.RegisterBudgetServiceServer(srv, g)
	reflection.Register(srv)

	g.logger.Info("budget gRPC server listening", "addr", addr)
	return srv.Serve(lis)
}
