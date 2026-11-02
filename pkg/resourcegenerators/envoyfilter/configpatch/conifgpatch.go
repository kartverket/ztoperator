package configpatch

import (
	"fmt"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
)

func ToPbStruct(msg proto.Message) (*structpb.Struct, error) {
	b, err := protojson.MarshalOptions{UseProtoNames: true}.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("marshal %T: %w", msg, err)
	}
	s := &structpb.Struct{}
	if unmarshalErr := protojson.Unmarshal(b, s); unmarshalErr != nil {
		return nil, fmt.Errorf("unmarshal %T into struct: %w", msg, unmarshalErr)
	}
	return s, nil
}
