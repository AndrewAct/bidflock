// Package budget contains gRPC service types for the Budget service.
// These types mirror what protoc would generate from proto/budget.proto.
// Regenerate with: make proto-gen
package budget

import (
	"context"

	"google.golang.org/grpc"
)

// --- Message types ---

type CheckBudgetRequest struct {
	CampaignID string  `json:"campaign_id"`
	UserID     string  `json:"user_id"`
	BidAmount  float64 `json:"bid_amount"`
}

type CheckBudgetResponse struct {
	Allowed              bool    `json:"allowed"`
	Reason               string  `json:"reason,omitempty"`
	RemainingDailyBudget float64 `json:"remaining_daily_budget"`
	PacingMultiplier     float64 `json:"pacing_multiplier"` // 0.0-1.0
}

type DeductBudgetRequest struct {
	CampaignID string  `json:"campaign_id"`
	BidID      string  `json:"bid_id"`
	Amount     float64 `json:"amount"`
}

type DeductBudgetResponse struct {
	Success              bool    `json:"success"`
	RemainingDailyBudget float64 `json:"remaining_daily_budget"`
}

type PacingRequest struct {
	CampaignID string `json:"campaign_id"`
}

type PacingResponse struct {
	SpendRate         float64 `json:"spend_rate"`
	TargetRate        float64 `json:"target_rate"`
	PacingMultiplier  float64 `json:"pacing_multiplier"`
	DailySpendSoFar   float64 `json:"daily_spend_so_far"`
	DailyBudget       float64 `json:"daily_budget"`
}

// --- Service interfaces ---

type BudgetServiceServer interface {
	CheckBudget(context.Context, *CheckBudgetRequest) (*CheckBudgetResponse, error)
	DeductBudget(context.Context, *DeductBudgetRequest) (*DeductBudgetResponse, error)
	GetPacingInfo(context.Context, *PacingRequest) (*PacingResponse, error)
}

type BudgetServiceClient interface {
	CheckBudget(ctx context.Context, in *CheckBudgetRequest, opts ...grpc.CallOption) (*CheckBudgetResponse, error)
	DeductBudget(ctx context.Context, in *DeductBudgetRequest, opts ...grpc.CallOption) (*DeductBudgetResponse, error)
	GetPacingInfo(ctx context.Context, in *PacingRequest, opts ...grpc.CallOption) (*PacingResponse, error)
}

// --- Client implementation ---

type budgetServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBudgetServiceClient(cc grpc.ClientConnInterface) BudgetServiceClient {
	return &budgetServiceClient{cc}
}

func (c *budgetServiceClient) CheckBudget(ctx context.Context, in *CheckBudgetRequest, opts ...grpc.CallOption) (*CheckBudgetResponse, error) {
	out := new(CheckBudgetResponse)
	err := c.cc.Invoke(ctx, "/budget.BudgetService/CheckBudget", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *budgetServiceClient) DeductBudget(ctx context.Context, in *DeductBudgetRequest, opts ...grpc.CallOption) (*DeductBudgetResponse, error) {
	out := new(DeductBudgetResponse)
	err := c.cc.Invoke(ctx, "/budget.BudgetService/DeductBudget", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *budgetServiceClient) GetPacingInfo(ctx context.Context, in *PacingRequest, opts ...grpc.CallOption) (*PacingResponse, error) {
	out := new(PacingResponse)
	err := c.cc.Invoke(ctx, "/budget.BudgetService/GetPacingInfo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- Server registration ---

func RegisterBudgetServiceServer(s grpc.ServiceRegistrar, srv BudgetServiceServer) {
	s.RegisterService(&BudgetService_ServiceDesc, srv)
}

var BudgetService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "budget.BudgetService",
	HandlerType: (*BudgetServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CheckBudget",
			Handler:    _BudgetService_CheckBudget_Handler,
		},
		{
			MethodName: "DeductBudget",
			Handler:    _BudgetService_DeductBudget_Handler,
		},
		{
			MethodName: "GetPacingInfo",
			Handler:    _BudgetService_GetPacingInfo_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "budget.proto",
}

func _BudgetService_CheckBudget_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckBudgetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BudgetServiceServer).CheckBudget(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/budget.BudgetService/CheckBudget"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BudgetServiceServer).CheckBudget(ctx, req.(*CheckBudgetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BudgetService_DeductBudget_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeductBudgetRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BudgetServiceServer).DeductBudget(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/budget.BudgetService/DeductBudget"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BudgetServiceServer).DeductBudget(ctx, req.(*DeductBudgetRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BudgetService_GetPacingInfo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(PacingRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BudgetServiceServer).GetPacingInfo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/budget.BudgetService/GetPacingInfo"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BudgetServiceServer).GetPacingInfo(ctx, req.(*PacingRequest))
	}
	return interceptor(ctx, in, info, handler)
}
