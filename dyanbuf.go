package dynabuf

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Error represents a dynabuf error message that can be used
// for robust error handling when marshaling and unmarshaling
// between protobuf messages and DynamoDB attribute values.
//
// # Example
//
//	import (
//	  "fmt"
//	  "errors"
//
//	  "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
//	  "github.com/picatz/dynabuf"
//	)
//
//	av := map[string]types.AttributeValue{
//	  "bar": &types.AttributeValueMemberS{
//	    Value: "hello world",
//	  },
//	}
//
//	var output structpb.Struct
//	err := dynabuf.Unmarshal(av, &output)
//	if err != nil {
//	  switch {
//	  case errors.Is(err, dynabuf.ErrFailedToUnmarshal):
//	    fmt.Println(err)
//	  case errors.Is(err, dynabuf.ErrFailedToUnmarshalIntermediary):
//	    fmt.Println(err)
//	  default:
//	    fmt.Println("unknown error:", err)
//	  }
//	}
type Error string

// Error implements the [error] interface.
//
// [error]: https://pkg.go.dev/builtin#error
func (e Error) Error() string { return "dynabuf: " + string(e) }

// Set of error messages that can be returned by the Marshal and Unmarshal functions.
var (
	// ErrFailedToMarshal is returned when the function fails to marshal a protobuf message to a DynamoDB attribute value.
	ErrFailedToMarshal = Error("failed to marshal protobuf to DynamoDB attribute value")

	// ErrFailedToMarshalIntermediary is returned when the function fails to marshal the intermediary map to JSON.
	ErrFailedToMarshalIntermediary = Error("failed to marshal intermediary map to JSON")

	// ErrFailedToUnmarshalIntermediary is returned when the function fails to unmarshal the DynamoDB attribute value to an intermediary map.
	ErrFailedToUnmarshalIntermediary = Error("failed to unmarshal DynamoDB attribute value to intermediary map")

	// ErrFailedToUnmarshal is returned when the function fails to unmarshal a DynamoDB attribute value to a protobuf message.
	ErrFailedToUnmarshal = Error("failed to unmarshal DynamoDB attribute value to protobuf")

	// ErrInvalidInput is returned when the input is not a protobuf message or slice of messages.
	ErrInvalidInput = Error("invalid input, must be a protobuf message or slice of messages")

	// ErrInvalidOutput is returned when the output is not a pointer to a protobuf message or slice of messages.
	ErrInvalidOutput = Error("invalid output, must be a pointer to a protobuf message or slice of messages")
)

// Marshal returns the [DynamoDB] attribute value encoding of the given
// protobuf message or slice of messages. If there are any issues with
// marshaling, an error is returned.
//
// # Protocol Buffer to DynamoDB Attribute Value Marshaling
//
// We use a three-step process to marshal a protobuf message to a DynamoDB
// [attribute value], using JSON as the logical intermediary.
//
//  1. The function first marshals the protobuf message to [JSON] using the
//     [google.golang.org/protobuf/encoding/protojson] package.
//
//  2. Then, it unmarshals the JSON to a map using the standard library's
//     [encoding/json] package.
//
//  3. Finally, it uses the [github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue]
//     package to marshal the map to a DynamoDB attribute value.
//
// The process is similar for a slice of protobuf messages, but the function
// iterates over each message in the slice and marshals them individually.
//
// # Example
//
//	import (
//	  "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
//	  "google.golang.org/protobuf/types/known/structpb"
//	  "github.com/picatz/dynabuf"
//	)
//
//	input := &structpb.Struct{
//	  Fields: map[string]*structpb.Value{
//	    "bar": {
//	      Kind: &structpb.Value_StringValue{
//	        StringValue: "hello world",
//	      },
//	    },
//	  },
//	}
//
//	outputAny, _ := dynabuf.Marshal(input)
//
//	output := outputAny.(map[string]types.AttributeValue)
//
// [DynamoDB]: https://aws.amazon.com/dynamodb/
// [attribute value]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_AttributeValue.html
// [JSON]: https://protobuf.dev/programming-guides/proto3/#json
func Marshal(v any) (any, error) {
	if reflect.ValueOf(v).Kind() == reflect.Slice {
		return marshalProtoSlice(v)
	}

	return marshalProtoMessage(v)
}

// marshalProtoMessage handles marshaling of a single protobuf message
// to a DynamoDB attribute value. It returns the DynamoDB attribute value
// map or an error if there are any issues.
func marshalProtoMessage(v any) (map[string]types.AttributeValue, error) {
	if !isProtoMessage(v) {
		return nil, fmt.Errorf("%w: %w: %T", ErrFailedToMarshal, ErrInvalidInput, v)
	}

	b, err := protojson.Marshal(v.(proto.Message))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	var intermediary map[string]any
	err = json.Unmarshal(b, &intermediary)
	if err != nil {
		return nil, fmt.Errorf("%w: %w: %w", ErrFailedToMarshal, ErrFailedToUnmarshalIntermediary, err)
	}

	av, err := attributevalue.MarshalMap(intermediary)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFailedToMarshal, err)
	}

	return av, nil
}

// marshalProtoSlice handles marshaling of a slice of protobuf messages to
// a slice of DynamoDB attribute values. It returns the DynamoDB attribute
// value slice or an error if there are any issues.
func marshalProtoSlice(v any) ([]map[string]types.AttributeValue, error) {
	sliceValue := reflect.ValueOf(v)
	if sliceValue.Kind() != reflect.Slice {
		return nil, fmt.Errorf("%w: %w: %T", ErrFailedToMarshal, ErrInvalidInput, v)
	}

	if !isProtoSlice(sliceValue) {
		return nil, fmt.Errorf("%w: %w: %T", ErrFailedToMarshal, ErrInvalidInput, v)
	}

	result := make([]map[string]types.AttributeValue, sliceValue.Len())

	for i := 0; i < sliceValue.Len(); i++ {
		item := sliceValue.Index(i).Interface()
		av, err := marshalProtoMessage(item)
		if err != nil {
			return nil, fmt.Errorf("%w: at index %d: %w", ErrFailedToMarshal, i, err)
		}
		result[i] = av
	}

	return result, nil
}

// isProtoMessage checks if the given any is a protobuf message type
// by asserting it as a [proto.Message] interface.
//
// [proto.Message]: https://pkg.go.dev/google.golang.org/protobuf/proto#Message
func isProtoMessage(v any) bool {
	_, ok := v.(proto.Message)
	return ok
}

// isProtoSlice checks if the given reflect.Value is a slice of protobuf messages
// by checking if the element type is a pointer to a [proto.Message].
//
// [proto.Message]: https://pkg.go.dev/google.golang.org/protobuf/proto#Message
func isProtoSlice(v reflect.Value) bool {
	if v.Kind() != reflect.Slice {
		return false
	}
	elemType := v.Type().Elem()
	if elemType.Kind() != reflect.Ptr {
		return false
	}
	return isProtoMessage(reflect.New(elemType.Elem()).Interface())
}

// Unmarshal parses the [DynamoDB] attribute values in av and stores the result in v.
// v must be a pointer to a single protobuf message or a slice of protobuf messages.
// If there are any issues with unmarshaling, an error is returned.
//
// # DynamoDB Attribute Value to Protocol Buffer Unmarshaling
//
// We use a three-step process to unmarshal a DynamoDB [attribute value] to a
// protobuf message, using [JSON] as the logical intermediary.
//
//  1. The function first unmarshals the DynamoDB attribute value to a map using
//     the [github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue] package.
//
//  2. Then, it marshals the map to JSON using the standard library's
//     [encoding/json] package.
//
//  3. Finally, it unmarshals the JSON to a protobuf message using the
//     [google.golang.org/protobuf/encoding/protojson] package.
//
// The process is similar for a slice of protobuf messages, but the function
// iterates over each item in the slice and unmarshals them individually.
//
// # Example
//
//	import (
//	  "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
//	  "google.golang.org/protobuf/types/known/structpb"
//	  "github.com/picatz/dynabuf"
//	)
//
//	av := map[string]types.AttributeValue{
//	  "bar": &types.AttributeValueMemberS{
//	    Value: "hello world",
//	  },
//	}
//
//	var output structpb.Struct
//	_ = dynabuf.Unmarshal(av, &output)
//
// [DynamoDB]: https://aws.amazon.com/dynamodb/
// [attribute value]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_AttributeValue.html
// [JSON]: https://protobuf.dev/programming-guides/proto3/#json
func Unmarshal(av any, v any) error {
	vValue := reflect.ValueOf(v)
	if vValue.Kind() != reflect.Ptr {
		return fmt.Errorf("%w: %w: %T", ErrFailedToUnmarshal, ErrInvalidOutput, v)
	}
	vElem := vValue.Elem()
	isSlice := vElem.Kind() == reflect.Slice

	if isSlice && !isProtoSlice(vElem) || !isSlice && !isProtoMessage(v) {
		return fmt.Errorf("%w: %w: %T", ErrFailedToUnmarshal, ErrInvalidOutput, v)
	}

	var intermediateValue any
	switch typedAV := av.(type) {
	case types.AttributeValue:
		intermediateValue = make(map[string]any)
		err := attributevalue.Unmarshal(typedAV, &intermediateValue)
		if err != nil {
			return fmt.Errorf("%w: failed to unmarshal DynamoDB attribute value: %w", ErrFailedToUnmarshal, err)
		}
	case map[string]types.AttributeValue:
		intermediateValue = make(map[string]any)
		err := attributevalue.UnmarshalMap(typedAV, &intermediateValue)
		if err != nil {
			return fmt.Errorf("%w: failed to unmarshal DynamoDB attribute map: %w", ErrFailedToUnmarshal, err)
		}
	case []map[string]types.AttributeValue:
		if !isSlice {
			return fmt.Errorf("%w: %w: %T", ErrFailedToUnmarshal, ErrInvalidOutput, v)
		}
		intermediateValue = make([]map[string]any, len(typedAV))
		for i, item := range typedAV {
			err := attributevalue.UnmarshalMap(item, &intermediateValue.([]map[string]any)[i])
			if err != nil {
				return fmt.Errorf("%w: failed to unmarshal DynamoDB attribute map: %w", ErrFailedToUnmarshal, err)
			}
		}
	default:
		return fmt.Errorf("%w: %w: unsupported type: %T", ErrFailedToUnmarshal, ErrInvalidOutput, v)
	}

	intermediateBytes, err := json.Marshal(intermediateValue)
	if err != nil {
		return fmt.Errorf("%w: %w: %w", ErrFailedToUnmarshal, ErrFailedToMarshalIntermediary, err)
	}

	if isSlice {
		err = unmarshalJSONToProtoSlice(intermediateBytes, v)
	} else {
		err = protojson.Unmarshal(intermediateBytes, v.(proto.Message))
	}
	if err != nil {
		return fmt.Errorf("%w: %w: %w", ErrFailedToUnmarshal, ErrFailedToUnmarshalIntermediary, err)
	}

	return nil
}

// unmarshalJSONToProtoSlice unmarshals JSON data to a slice of protobuf messages
func unmarshalJSONToProtoSlice(data []byte, v any) error {
	slice := reflect.ValueOf(v).Elem()
	var jsonSlice []json.RawMessage
	if err := json.Unmarshal(data, &jsonSlice); err != nil {
		return err
	}

	for _, item := range jsonSlice {
		elemType := slice.Type().Elem()
		elem := reflect.New(elemType.Elem()).Interface().(proto.Message)
		if err := protojson.Unmarshal(item, elem); err != nil {
			return err
		}
		slice.Set(reflect.Append(slice, reflect.ValueOf(elem)))
	}

	return nil
}

// Updates translates the given DynamoDB attribute value map to an update map.
// This is useful when updating an item in a DynamoDB table.
func Updates(mav map[string]types.AttributeValue) map[string]types.AttributeValueUpdate {
	updates := make(map[string]types.AttributeValueUpdate, len(mav))
	for k, v := range mav {
		updates[k] = types.AttributeValueUpdate{
			Value:  v,
			Action: types.AttributeActionPut,
		}
	}
	return updates
}
