// Package scoring contains gRPC service types for the Scoring service.
// These types mirror what protoc would generate from proto/scoring.proto.
// Regenerate with: make proto-gen
package scoring

import (
	"context"

	"google.golang.org/grpc"
)

// --- Message types ---

type BidRequest struct {
	ID     string   `json:"id"`
	UserID string   `json:"user_id"`
	Geo    string   `json:"geo"`
	DeviceType string `json:"device_type"`
	SSPID  string   `json:"ssp_id"`
	Interests []string `json:"interests,omitempty"`
}

type ScoreRequest struct {
	RequestID   string     `json:"request_id"`
	BidRequest  BidRequest `json:"bid_request"`
	CampaignIDs []string   `json:"campaign_ids"`
}

type ScoreResponse struct {
	RequestID string     `json:"request_id"`
	Scores    []AdScore  `json:"scores"`
}

type AdScore struct {
	CampaignID   string  `json:"campaign_id"`
	AdID         string  `json:"ad_id"`
	PredictedCTR float64 `json:"predicted_ctr"`
	PredictedCVR float64 `json:"predicted_cvr"`
	EffectiveBid float64 `json:"effective_bid"`
	QualityScore float64 `json:"quality_score"`
}

// --- Service interfaces ---

type ScoringServiceServer interface {
	ScoreAds(context.Context, *ScoreRequest) (*ScoreResponse, error)
}

type ScoringServiceClient interface {
	ScoreAds(ctx context.Context, in *ScoreRequest, opts ...grpc.CallOption) (*ScoreResponse, error)
}

// --- Client implementation ---

type scoringServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewScoringServiceClient(cc grpc.ClientConnInterface) ScoringServiceClient {
	return &scoringServiceClient{cc}
}

func (c *scoringServiceClient) ScoreAds(ctx context.Context, in *ScoreRequest, opts ...grpc.CallOption) (*ScoreResponse, error) {
	out := new(ScoreResponse)
	err := c.cc.Invoke(ctx, "/scoring.ScoringService/ScoreAds", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// --- Server registration ---

func RegisterScoringServiceServer(s grpc.ServiceRegistrar, srv ScoringServiceServer) {
	s.RegisterService(&ScoringService_ServiceDesc, srv)
}

var ScoringService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "scoring.ScoringService",
	HandlerType: (*ScoringServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ScoreAds",
			Handler:    _ScoringService_ScoreAds_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "scoring.proto",
}

func _ScoringService_ScoreAds_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ScoreRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ScoringServiceServer).ScoreAds(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/scoring.ScoringService/ScoreAds"}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ScoringServiceServer).ScoreAds(ctx, req.(*ScoreRequest))
	}
	return interceptor(ctx, in, info, handler)
}
