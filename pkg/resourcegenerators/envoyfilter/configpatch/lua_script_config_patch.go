package configpatch

import (
	"fmt"

	"github.com/kartverket/ztoperator/pkg/envoy/httpfilter"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/structpb"
)

func GetLuaScriptConfigPatchValue(luaScript string) (*structpb.Struct, error) {
	lua, err := httpfilter.LuaHttpFilter(luaScript)
	luaAny, err := anypb.New(lua)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize lua HTTP filter to protobuf Any: %w", err)
	}
	patchValue, err := ToPbStruct(luaAny)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize lua HTTP filter config patch value to protobuf struct: %w", err)
	}
	return patchValue, nil
}
