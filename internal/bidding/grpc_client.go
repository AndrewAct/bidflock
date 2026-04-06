package bidding

import (
	"context"
	"fmt"
	"time"

	budgetpb "github.com/AndrewAct/bidflock/gen/go/budget"
	scoringpb "github.com/AndrewAct/bidflock/gen/go/scoring"
	_ "github.com/AndrewAct/bidflock/pkg/codec"
	"github.com/AndrewAct/bidflock/pkg/observability"
	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ScoringClient struct {
	client scoringpb.ScoringServiceClient
}

func NewScoringClient(addr string) (*ScoringClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("scoring grpc dial: %w", err)
	}
	return &ScoringClient{client: scoringpb.NewScoringServiceClient(conn)}, nil
}

func (sc *ScoringClient) ScoreAds(ctx context.Context, req *scoringpb.ScoreRequest) (*scoringpb.ScoreResponse, error) {
	start := time.Now()
	resp, err := sc.client.ScoreAds(ctx, req)
	status := "ok"
	if err != nil {
		status = "error"
	}
	observability.GRPCRequestDuration.With(prometheus.Labels{
		"service": "scoring",
		"method":  "ScoreAds",
		"status":  status,
	}).Observe(time.Since(start).Seconds())
	return resp, err
}

type BudgetClient struct {
	client budgetpb.BudgetServiceClient
}

func NewBudgetClient(addr string) (*BudgetClient, error) {
	conn, err := grpc.NewClient(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("budget grpc dial: %w", err)
	}
	return &BudgetClient{client: budgetpb.NewBudgetServiceClient(conn)}, nil
}

func (bc *BudgetClient) CheckBudget(ctx context.Context, req *budgetpb.CheckBudgetRequest) (*budgetpb.CheckBudgetResponse, error) {
	start := time.Now()
	resp, err := bc.client.CheckBudget(ctx, req)
	status := "ok"
	if err != nil {
		status = "error"
	}
	observability.GRPCRequestDuration.With(prometheus.Labels{
		"service": "budget",
		"method":  "CheckBudget",
		"status":  status,
	}).Observe(time.Since(start).Seconds())
	return resp, err
}

func (bc *BudgetClient) DeductBudget(ctx context.Context, req *budgetpb.DeductBudgetRequest) (*budgetpb.DeductBudgetResponse, error) {
	return bc.client.DeductBudget(ctx, req)
}
