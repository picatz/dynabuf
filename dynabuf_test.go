package dynabuf_test

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/picatz/dynabuf"
	"github.com/shoenig/test/must"
	"google.golang.org/protobuf/types/known/structpb"
)

// TestMarshal tests the Marshal function with a struct from the
// [google.golang.org/protobuf/types/known/structpb] package.
//
// This is a common, protobuf defined struct that is used in many Go projects.
// It primarily allows us to avoid generating our own Go structs for this test.
func TestMarshal(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, input, output any, err error)
	}{
		{
			name: "single struct",
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"bar": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello world",
						},
					},
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputStruct, ok := input.(*structpb.Struct)
				must.True(t, ok)

				outputMap, ok := output.(map[string]types.AttributeValue)
				must.True(t, ok)
				must.Eq(t, len(inputStruct.Fields), len(outputMap))
				must.MapContainsKeys(t, outputMap, []string{"bar"})
				must.Eq(t, inputStruct.Fields["bar"].GetStringValue(), outputMap["bar"].(*types.AttributeValueMemberS).Value)
			},
		},
		{
			name: "list of structs",
			input: []*structpb.Struct{
				{
					Fields: map[string]*structpb.Value{
						"foo": {
							Kind: &structpb.Value_StringValue{
								StringValue: "hello world",
							},
						},
					},
				},
				{
					Fields: map[string]*structpb.Value{
						"bar": {
							Kind: &structpb.Value_StringValue{
								StringValue: "hello moon",
							},
						},
					},
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputList, ok := input.([]*structpb.Struct)
				must.True(t, ok)

				outputList, ok := output.([]map[string]types.AttributeValue)
				must.True(t, ok)
				must.Eq(t, len(inputList), len(outputList))
				must.MapContainsKeys(t, outputList[0], []string{"foo"})
				must.MapContainsKeys(t, outputList[1], []string{"bar"})
				must.Eq(t, inputList[0].Fields["foo"].GetStringValue(), outputList[0]["foo"].(*types.AttributeValueMemberS).Value)
				must.Eq(t, inputList[1].Fields["bar"].GetStringValue(), outputList[1]["bar"].(*types.AttributeValueMemberS).Value)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := dynabuf.Marshal(test.input)
			test.check(t, test.input, out, err)
		})
	}
}

func TestUnmarshal(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, input, output any, err error)
	}{
		{
			name: "single struct",
			input: map[string]types.AttributeValue{
				"bar": &types.AttributeValueMemberS{
					Value: "hello world",
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputMap, ok := input.(map[string]types.AttributeValue)
				must.True(t, ok)

				outputStruct := &structpb.Struct{}
				err = dynabuf.Unmarshal(inputMap, outputStruct)
				must.NoError(t, err)
				must.Eq(t, 1, len(outputStruct.Fields))
				must.MapContainsKeys(t, outputStruct.Fields, []string{"bar"})
				must.Eq(t, inputMap["bar"].(*types.AttributeValueMemberS).Value, outputStruct.Fields["bar"].GetStringValue())
			},
		},
		{
			name: "list of structs",
			input: []map[string]types.AttributeValue{
				{
					"foo": &types.AttributeValueMemberS{
						Value: "hello world",
					},
				},
				{
					"bar": &types.AttributeValueMemberS{
						Value: "hello moon",
					},
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputList, ok := input.([]map[string]types.AttributeValue)
				must.True(t, ok)

				outputList := []*structpb.Struct{}
				err = dynabuf.Unmarshal(inputList, &outputList)
				must.NoError(t, err)
				must.Eq(t, 2, len(outputList))
				must.MapContainsKeys(t, outputList[0].Fields, []string{"foo"})
				must.MapContainsKeys(t, outputList[1].Fields, []string{"bar"})
				must.Eq(t, inputList[0]["foo"].(*types.AttributeValueMemberS).Value, outputList[0].Fields["foo"].GetStringValue())
				must.Eq(t, inputList[1]["bar"].(*types.AttributeValueMemberS).Value, outputList[1].Fields["bar"].GetStringValue())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var (
				out any
				err error
			)
			switch test.input.(type) {
			case map[string]types.AttributeValue:

				mapOut := &structpb.Struct{}
				err = dynabuf.Unmarshal(test.input, mapOut)

				out = mapOut
			case []map[string]types.AttributeValue:
				listOut := []*structpb.Struct{}
				err = dynabuf.Unmarshal(test.input, &listOut)

				out = listOut
			default:
				t.Fatalf("unknown type: %T", test.input)
			}

			test.check(t, test.input, out, err)
		})
	}
}

// TestRoudtrip tests the Marshal and Unmarshal functions with a struct from the
// [google.golang.org/protobuf/types/known/structpb] package in a roundtrip
// fashion, such that the output of the Marshal function is passed to the
// Unmarshal function and the original input values are compared to the
// output values.
func TestRoudtrip(t *testing.T) {
	tests := []struct {
		name  string
		input any
		check func(t *testing.T, input, output any, err error)
	}{
		{
			name: "single struct",
			input: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"bar": {
						Kind: &structpb.Value_StringValue{
							StringValue: "hello world",
						},
					},
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputStruct, ok := input.(*structpb.Struct)
				must.True(t, ok)

				outputStruct, ok := output.(*structpb.Struct)
				must.True(t, ok)

				must.Eq(t, len(inputStruct.Fields), len(outputStruct.Fields))
				must.MapContainsKeys(t, outputStruct.Fields, []string{"bar"})
				must.Eq(t, inputStruct.Fields["bar"].GetStringValue(), outputStruct.Fields["bar"].GetStringValue())
			},
		},
		{
			name: "list of structs",
			input: []*structpb.Struct{
				{
					Fields: map[string]*structpb.Value{
						"foo": {
							Kind: &structpb.Value_StringValue{
								StringValue: "hello world",
							},
						},
					},
				},
				{
					Fields: map[string]*structpb.Value{
						"bar": {
							Kind: &structpb.Value_StringValue{
								StringValue: "hello moon",
							},
						},
					},
				},
			},
			check: func(t *testing.T, input, output any, err error) {
				must.NoError(t, err)

				inputList, ok := input.([]*structpb.Struct)
				must.True(t, ok)

				outputList, ok := output.([]*structpb.Struct)
				must.True(t, ok)

				must.Eq(t, len(inputList), len(outputList))
				must.MapContainsKeys(t, outputList[0].Fields, []string{"foo"})
				must.MapContainsKeys(t, outputList[1].Fields, []string{"bar"})
				must.Eq(t, inputList[0].Fields["foo"].GetStringValue(), outputList[0].Fields["foo"].GetStringValue())
				must.Eq(t, inputList[1].Fields["bar"].GetStringValue(), outputList[1].Fields["bar"].GetStringValue())
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			outAny, err := dynabuf.Marshal(test.input)
			must.NoError(t, err)

			var (
				out any
			)

			switch test.input.(type) {
			case *structpb.Struct:
				outMap, ok := outAny.(map[string]types.AttributeValue)
				must.True(t, ok)

				outStruct := &structpb.Struct{}
				err = dynabuf.Unmarshal(outMap, outStruct)
				out = outStruct
			case []*structpb.Struct:
				outList, ok := outAny.([]map[string]types.AttributeValue)
				must.True(t, ok)

				outStructList := []*structpb.Struct{}
				err = dynabuf.Unmarshal(outList, &outStructList)
				out = outStructList
			default:
				t.Fatalf("unknown type: %T", test.input)
			}

			test.check(t, test.input, out, err)
		})
	}
}
