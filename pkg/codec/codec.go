// Package codec registers a JSON codec for gRPC, replacing the default protobuf codec.
// This lets us use plain Go structs as gRPC message types without running protoc.
// Proto files in proto/ still define the canonical schema and can generate real code later.
package codec

import (
	"encoding/json"

	"google.golang.org/grpc/encoding"
)

const Name = "proto"

func init() {
	encoding.RegisterCodec(JSON{})
}

// JSON is a gRPC codec that uses encoding/json.
type JSON struct{}

func (JSON) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (JSON) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (JSON) Name() string { return Name }
