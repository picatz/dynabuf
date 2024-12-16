package expr_test

import (
	"context"
	"fmt"
	"net"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/google/cel-go/cel"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/picatz/dynabuf"
	"github.com/picatz/dynabuf/expr"
	"github.com/shoenig/test/must"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Demonstrates converting a CEL AST to a DynamoDB Condition expression.
func ExampleCondition() {
	env, err := expr.NewEnv(
		cel.Variable("name", celtypes.StringType),
		cel.Variable("tags", celtypes.NewListType(celtypes.StringType)),
	)
	if err != nil {
		panic(err)
	}

	ast, issues := env.Compile(`name == "bob" && "urgent" in tags`)
	if issues != nil && issues.Err() != nil {
		panic(issues.Err())
	}

	cond, err := expr.Condition(ast)
	if err != nil {
		panic(err)
	}

	dynamoExpr, err := expression.NewBuilder().WithCondition(cond).Build()
	if err != nil {
		panic(err)
	}

	fmt.Println(dynamoExpr.Condition())
	fmt.Println(dynamoExpr.Names())
	fmt.Println(len(dynamoExpr.Values()))
	fmt.Println(dynamoExpr.Values()[":0"])
	fmt.Println(dynamoExpr.Values()[":1"])
	// Output:
	// (#0 = :0) AND (:1 IN (#1))
	// map[#0:name #1:tags]
	// 2
	// &{bob {}}
	// &{urgent {}}
}

// Demonstrates converting a CEL AST into a DynamoDB key condition expression.
func ExampleKeyCondition() {
	env, err := expr.NewEnv(
		cel.Variable("user", celtypes.StringType),  // partition key
		cel.Variable("order", celtypes.StringType), // sort key
	)
	if err != nil {
		panic(err)
	}

	ast, issues := env.Compile(`user == "User#123" && begins_with(order, "Order#")`)
	if issues != nil && issues.Err() != nil {
		panic(issues.Err())
	}

	keyCond, err := expr.KeyCondition(ast)
	if err != nil {
		panic(err)
	}

	keyCondExpr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
	if err != nil {
		panic(err)
	}

	fmt.Println(*keyCondExpr.KeyCondition())
	fmt.Println(keyCondExpr.Names())
	fmt.Println(len(keyCondExpr.Values()))
	fmt.Println(keyCondExpr.Values()[":0"])
	// Output:
	// (#0 = :0) AND (begins_with (#1, :1))
	// map[#0:user #1:order]
	// 2
	// &{User#123 {}}
}

// Demonstrates converting a CEL AST into a DynamoDB filter expression.
func ExampleFilter() {
	env, err := expr.NewEnv(
		cel.Variable("severity", celtypes.StringType),
		cel.Variable("status", celtypes.StringType),
		cel.Variable("tags", celtypes.NewListType(celtypes.StringType)),
	)
	if err != nil {
		panic(err)
	}

	ast, issues := env.Compile(`severity == "critical" && status == "open"`)
	if issues != nil && issues.Err() != nil {
		panic(issues.Err())
	}

	cond, err := expr.Filter(ast)
	if err != nil {
		panic(err)
	}

	condExpr, err := expression.NewBuilder().WithFilter(cond).Build()
	if err != nil {
		panic(err)
	}

	_ = dynamodb.QueryInput{
		TableName:                 aws.String("example-table"),
		FilterExpression:          condExpr.Filter(),
		ExpressionAttributeNames:  condExpr.Names(),
		ExpressionAttributeValues: condExpr.Values(),
	}

	fmt.Println(*condExpr.Filter())
	fmt.Println(condExpr.Names())
	fmt.Println(len(condExpr.Values()))
	fmt.Println(condExpr.Values()[":0"])
	fmt.Println(condExpr.Values()[":1"])
	// Output:
	// (#0 = :0) AND (#1 = :1)
	// map[#0:severity #1:status]
	// 2
	// &{critical {}}
	// &{open {}}
}

func ExampleProjection() {
	env, err := expr.NewEnv(
		cel.Variable("name", celtypes.StringType),
		cel.Variable("age", celtypes.IntType),
		cel.Variable("email", celtypes.StringType),
	)
	if err != nil {
		panic(err)
	}

	ast, issues := env.Compile(`["name", "age", "email"]`)
	if issues != nil && issues.Err() != nil {
		panic(issues.Err())
	}

	proj, err := expr.Projection(ast)
	if err != nil {
		panic(err)
	}

	projExpr, err := expression.NewBuilder().WithProjection(proj).Build()
	if err != nil {
		panic(err)
	}

	_ = dynamodb.QueryInput{
		TableName:                aws.String("example-table"),
		ProjectionExpression:     projExpr.Projection(),
		ExpressionAttributeNames: projExpr.Names(),
	}

	fmt.Println(*projExpr.Projection())
	fmt.Println(projExpr.Names())
	// Output:
	// #0, #1, #2
	// map[#0:name #1:age #2:email]
}

func ExampleUpdate() {
	env, err := expr.NewEnv(
		cel.Variable("views", celtypes.IntType),
		cel.Variable("likes", celtypes.IntType),
		cel.Variable("tags", celtypes.NewListType(celtypes.StringType)),
	)
	if err != nil {
		panic(err)
	}

	ast, issues := env.Compile(`add("views", 1)`)
	if issues != nil && issues.Err() != nil {
		panic(issues.Err())
	}

	update, err := expr.Update(ast)
	if err != nil {
		panic(err)
	}

	updateExpr, err := expression.NewBuilder().WithUpdate(update).Build()
	if err != nil {
		panic(err)
	}

	_ = dynamodb.UpdateItemInput{
		TableName:                 aws.String("example-table"),
		UpdateExpression:          updateExpr.Update(),
		ExpressionAttributeNames:  updateExpr.Names(),
		ExpressionAttributeValues: updateExpr.Values(),
	}

	fmt.Println(*updateExpr.Update())
	fmt.Println(updateExpr.Names())
	fmt.Println(len(updateExpr.Values()))
	fmt.Println(updateExpr.Values()[":0"])
	// Output:
	// ADD #0 :0
	//
	// map[#0:views]
	// 1
	// &{1 {}}
}

func TestMessageVariables(t *testing.T) {
	// Test using a proto message as a top-level variable in the CEL environment.
	tests := []struct {
		name  string
		msg   proto.Message
		check func(t *testing.T, opts []cel.EnvOption, env *cel.Env)
	}{
		{
			name: "structpb message",
			msg: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"severity": structpb.NewStringValue("critical"),
				},
			},
			check: func(t *testing.T, opts []cel.EnvOption, env *cel.Env) {
				must.Len(t, 2, opts) // cel.Types(msg) + cel.Variable(name, ...)
				ast, issues := env.Parse(`example.fields.severity == "critical"`)
				must.NoError(t, issues.Err())
				_, issues = env.Check(ast)
				must.NoError(t, issues.Err())
			},
		},
		{
			name: "structpb message with nested struct",
			msg: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"attributes": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"region": structpb.NewStringValue("us-east-1"),
						},
					}),
				},
			},
			check: func(t *testing.T, opts []cel.EnvOption, env *cel.Env) {
				must.Len(t, 2, opts)
				ast, issues := env.Parse(`example.fields.attributes.region == "us-east-1"`)
				must.NoError(t, issues.Err())
				_, issues = env.Check(ast)
				must.NoError(t, issues.Err())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := expr.MessageVariable("example", tt.msg)
			env, err := expr.NewEnv(opts...)
			must.NoError(t, err)
			tt.check(t, opts, env)
		})
	}
}

func TestMessageFieldVariables(t *testing.T) {
	// Test using message fields as variables in the CEL environment.
	tests := []struct {
		name  string
		msg   proto.Message
		check func(t *testing.T, opts []cel.EnvOption, env *cel.Env)
	}{
		{
			name: "structpb message single field",
			msg: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"severity": structpb.NewStringValue("critical"),
				},
			},
			check: func(t *testing.T, opts []cel.EnvOption, env *cel.Env) {
				must.Len(t, 1, opts)
				ast, issues := env.Parse(`severity == "critical"`)
				must.NoError(t, issues.Err())
				_, issues = env.Check(ast)
				must.NoError(t, issues.Err())
			},
		},
		{
			name: "structpb message with nested struct",
			msg: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"attributes": structpb.NewStructValue(&structpb.Struct{
						Fields: map[string]*structpb.Value{
							"region": structpb.NewStringValue("us-east-1"),
						},
					}),
				},
			},
			check: func(t *testing.T, opts []cel.EnvOption, env *cel.Env) {
				must.Len(t, 1, opts)
				ast, issues := env.Parse(`attributes.region == "us-east-1"`)
				must.NoError(t, issues.Err())
				_, issues = env.Check(ast)
				must.NoError(t, issues.Err())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := expr.MessageFieldVariables(tt.msg)
			env, err := expr.NewEnv(opts...)
			must.NoError(t, err)
			tt.check(t, opts, env)
		})
	}
}

func TestCondition(t *testing.T) {
	// Test converting CEL expressions to DynamoDB Condition expressions.
	env, err := expr.NewEnv(
		cel.Variable("severity", celtypes.StringType),
		cel.Variable("status", celtypes.StringType),
		cel.Variable("meta", celtypes.NewMapType(celtypes.StringType, celtypes.DynType)),
		cel.Variable("tags", celtypes.NewListType(celtypes.StringType)),
	)
	must.NoError(t, err)

	tests := []struct {
		name  string
		src   string
		check func(t *testing.T, dynamoExpr expression.Expression, err error)
	}{
		{
			name: "ident equals string",
			src:  `severity == "critical"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "#0 = :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "critical")
			},
		},
		{
			name: "begins_with function",
			src:  `begins_with(status, "open")`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "begins_with (#0, :0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "status"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "open")
			},
		},
		{
			name: "ident equals string",
			src:  `severity == "critical"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "#0 = :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "critical")
			},
		},
		{
			name: "ident logical AND condition",
			src:  `severity == "high" && status == "open"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "(#0 = :0) AND (#1 = :1)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity", "#1": "status"})
				must.MapContainsKey(t, expr.Values(), ":0")
				must.MapContainsKey(t, expr.Values(), ":1")
				value0, ok0 := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok0)
				must.Eq(t, value0.Value, "high")
				value1, ok1 := expr.Values()[":1"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok1)
				must.Eq(t, value1.Value, "open")
			},
		},
		{
			name: "ident logical OR condition",
			src:  `status == "closed" || status == "resolved"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "(#0 = :0) OR (#0 = :1)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "status"})
				must.MapContainsKey(t, expr.Values(), ":0")
				must.MapContainsKey(t, expr.Values(), ":1")
				value0, ok0 := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok0)
				must.Eq(t, value0.Value, "closed")
				value1, ok1 := expr.Values()[":1"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok1)
				must.Eq(t, value1.Value, "resolved")
			},
		},
		{
			name: "ident has field",
			src:  `has(meta.example)`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "attribute_exists (#0.#1)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "meta", "#1": "example"})
			},
		},
		{
			name: "negation condition",
			src:  `!(severity == "low")`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "NOT (#0 = :0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "low")
			},
		},
		{
			name: "ident size greater than int",
			src:  `size(tags) > 2`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "size (#0) > :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "tags"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberN)
				must.True(t, ok)
				must.Eq(t, value.Value, "2")
			},
		},
		{
			name: "ident attribute exists",
			src:  `attribute_exists(tags)`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "attribute_exists (#0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "tags"})
			},
		},
		{
			name: "ident in list",
			src:  `severity in ["critical", "high", "medium"]`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "#0 IN (:0, :1, :2)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity"})
				must.MapContainsKey(t, expr.Values(), ":0")
				must.MapContainsKey(t, expr.Values(), ":1")
				must.MapContainsKey(t, expr.Values(), ":2")
				value0, ok0 := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok0)
				must.Eq(t, value0.Value, "critical")
				value1, ok1 := expr.Values()[":1"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok1)
				must.Eq(t, value1.Value, "high")
				value2, ok2 := expr.Values()[":2"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok2)
				must.Eq(t, value2.Value, "medium")
			},
		},
		{
			name: "attribute not exists function",
			src:  `attribute_not_exists(meta["priority"])`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "attribute_not_exists (#0.#1)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "meta", "#1": "priority"})
			},
		},
		{
			name: "not attribute exists function",
			src:  `!attribute_exists(meta["priority"])`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "NOT (attribute_exists (#0.#1))")
				must.Eq(t, expr.Names(), map[string]string{"#0": "meta", "#1": "priority"})
			},
		},
		{
			name: "map element access",
			src:  `meta["region"] == "us-east-1"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "#0.#1 = :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "meta", "#1": "region"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "us-east-1")
			},
		},
		{
			name: "contains function on list",
			src:  `contains(tags, "urgent")`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "contains (#0, :0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "tags"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "urgent")
			},
		},
		{
			name: "begins_with function",
			src:  `begins_with(status, "open")`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "begins_with (#0, :0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "status"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "open")
			},
		},
		{
			name: "attribute_type function",
			src:  `attribute_type(severity, "S")`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				// DynamoDB requires the type parameter in attribute_type to be an expression attribute value.
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "attribute_type (#0, :0)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "severity"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, value.Value, "S")
			},
		},
		{
			name: "greater than comparison",
			src:  `size(tags) > 5`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "size (#0) > :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "tags"})
				must.MapContainsKey(t, expr.Values(), ":0")
				value, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberN)
				must.True(t, ok)
				must.Eq(t, value.Value, "5")
			},
		},
		{
			name: "between operator", // this isn't fully implemented by the converter (BETWEEN)
			src:  `size(tags) >= 1 && size(tags) <= 5`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Condition())
				must.Eq(t, *expr.Condition(), "(size (#0) >= :0) AND (size (#0) <= :1)")
				must.Eq(t, expr.Names(), map[string]string{"#0": "tags"})
				must.MapContainsKey(t, expr.Values(), ":0")
				must.MapContainsKey(t, expr.Values(), ":1")
				value0, ok0 := expr.Values()[":0"].(*dbtypes.AttributeValueMemberN)
				must.True(t, ok0)
				must.Eq(t, value0.Value, "1")
				value1, ok1 := expr.Values()[":1"].(*dbtypes.AttributeValueMemberN)
				must.True(t, ok1)
				must.Eq(t, value1.Value, "5")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.src)
			must.NoError(t, issues.Err())

			cond, err := expr.Condition(ast)
			must.NoError(t, err)

			dynamoExpr, err := expression.NewBuilder().WithCondition(cond).Build()
			must.NoError(t, err)

			tt.check(t, dynamoExpr, err)
		})
	}
}

func TestFilter(t *testing.T) {
	// Test converting CEL expressions to DynamoDB Filter expressions.
	env, err := expr.NewEnv(
		cel.Variable("category", celtypes.StringType),
		cel.Variable("price", celtypes.DoubleType),
		cel.Variable("ratings", celtypes.NewListType(celtypes.DoubleType)),
	)
	must.NoError(t, err)

	tests := []struct {
		name  string
		src   string
		check func(t *testing.T, expr expression.Expression, err error)
	}{
		{
			name: "filter category equals electronics",
			src:  `category == "electronics"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Filter())
				must.Eq(t, *expr.Filter(), "#0 = :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "category"})
				must.MapContainsKey(t, expr.Values(), ":0")
				val, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, val.Value, "electronics")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.src)
			must.NoError(t, issues.Err())

			cond, err := expr.Filter(ast)
			must.NoError(t, err)

			dynamoExpr, err := expression.NewBuilder().WithFilter(cond).Build()
			must.NoError(t, err)

			tt.check(t, dynamoExpr, err)
		})
	}
}

func TestKeyCondition(t *testing.T) {
	// Test converting CEL expressions into DynamoDB KeyCondition expressions.
	env, err := expr.NewEnv(
		cel.Variable("partitionKey", celtypes.StringType),
		cel.Variable("sortKey", celtypes.StringType),
	)
	must.NoError(t, err)

	tests := []struct {
		name  string
		src   string
		check func(t *testing.T, keyCond expression.Expression, err error)
	}{
		{
			name: "key condition equals",
			src:  `partitionKey == "User#123"`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.Eq(t, *expr.KeyCondition(), "#0 = :0")
				must.Eq(t, expr.Names(), map[string]string{"#0": "partitionKey"})
				must.MapContainsKey(t, expr.Values(), ":0")
				val, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberS)
				must.True(t, ok)
				must.Eq(t, val.Value, "User#123")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.src)
			must.NoError(t, issues.Err())

			keyCond, err := expr.KeyCondition(ast)
			must.NoError(t, err)

			keyCondExpr, err := expression.NewBuilder().WithKeyCondition(keyCond).Build()
			must.NoError(t, err)

			tt.check(t, keyCondExpr, err)
		})
	}
}

func TestProjection(t *testing.T) {
	// Test converting CEL expressions into DynamoDB Projection expressions.
	env, err := expr.NewEnv()
	must.NoError(t, err)

	tests := []struct {
		name  string
		src   string
		check func(t *testing.T, dynamoExpr expression.Expression, err error)
	}{
		{
			name: "simple projection",
			src:  `["name", "age", "email"]`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Projection())
				must.Eq(t, *expr.Projection(), "#0, #1, #2")
				must.MapEq(t, expr.Names(), map[string]string{"#0": "name", "#1": "age", "#2": "email"})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.src)
			must.NoError(t, issues.Err())

			proj, err := expr.Projection(ast)
			must.NoError(t, err)

			dynamoExpr, err := expression.NewBuilder().WithProjection(proj).Build()
			must.NoError(t, err)

			tt.check(t, dynamoExpr, err)
		})
	}
}

func TestUpdate(t *testing.T) {
	// Test converting CEL expressions into DynamoDB Update expressions.
	env, err := expr.NewEnv(
		cel.Variable("views", celtypes.IntType),
		cel.Variable("likes", celtypes.IntType),
		cel.Variable("tags", celtypes.NewListType(celtypes.StringType)),
	)
	must.NoError(t, err)

	tests := []struct {
		name  string
		src   string
		check func(t *testing.T, dynamoExpr expression.Expression, err error)
	}{
		{
			name: "increment views",
			src:  `add("views", 1)`,
			check: func(t *testing.T, expr expression.Expression, err error) {
				must.NoError(t, err)
				must.NotNil(t, expr.Update())
				must.Eq(t, *expr.Update(), "ADD #0 :0\n")
				must.Eq(t, expr.Names(), map[string]string{"#0": "views"})
				must.MapContainsKey(t, expr.Values(), ":0")
				val, ok := expr.Values()[":0"].(*dbtypes.AttributeValueMemberN)
				must.True(t, ok)
				must.Eq(t, val.Value, "1")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ast, issues := env.Compile(tt.src)
			must.NoError(t, issues.Err())

			update, err := expr.Update(ast)
			must.NoError(t, err)

			dynamoExpr, err := expression.NewBuilder().WithUpdate(update).Build()
			must.NoError(t, err)

			tt.check(t, dynamoExpr, err)
		})
	}
}

// awsConfigForLocalStack returns an AWS config pointing at a localstack instance for integration tests.
func awsConfigForLocalStack(ctx context.Context, t *testing.T, localstackContainer *localstack.LocalStackContainer) aws.Config {
	t.Helper()

	h, err := localstackContainer.Host(ctx)
	must.NoError(t, err, must.Sprint("failed to get localstack host"))

	p, err := localstackContainer.MappedPort(ctx, "4566")
	must.NoError(t, err, must.Sprint("failed to get mapped port for localstack"))

	localstackAddr := net.JoinHostPort(h, p.Port())

	// Ensure localstack is running
	localstackConn, err := net.Dial("tcp", localstackAddr)
	must.NoError(t, err, must.Sprint("ensure localstack container is running"))
	localstackConn.Close()

	cfg, err := config.LoadDefaultConfig(
		ctx,
		config.WithRegion("us-east-1"),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("X", "Y", "Z")),
		config.WithEndpointResolverWithOptions(
			aws.EndpointResolverWithOptionsFunc(func(service, region string, _ ...interface{}) (aws.Endpoint, error) {
				return aws.Endpoint{
					PartitionID:   "aws",
					URL:           "http://" + localstackAddr,
					SigningRegion: "us-east-1",
				}, nil
			}),
		),
	)
	must.NoError(t, err, must.Sprint("failed to load default config for localstack"))

	return cfg
}

func Test_WithLocalStack(t *testing.T) {
	// Integration test that uses a localstack container to test actual DynamoDB interactions.
	ctx := context.Background()
	tableName := aws.String("test-table")

	localstackContainer, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:1.4.0"),
		testcontainers.WithConfigModifier(func(config *container.Config) {
			config.ExposedPorts = nat.PortSet{"4566/tcp": struct{}{}}
		}),
		testcontainers.WithLogger(testcontainers.TestLogger(t)),
	)
	must.NoError(t, err, must.Sprint("failed to run localstack container"))

	t.Cleanup(func() {
		must.NoError(t, localstackContainer.Terminate(ctx), must.Sprint("failed to terminate localstack container"))
	})

	d := dynamodb.NewFromConfig(awsConfigForLocalStack(ctx, t, localstackContainer))

	// Create a test table
	_, err = d.CreateTable(ctx, &dynamodb.CreateTableInput{
		TableName: tableName,
		AttributeDefinitions: []dbtypes.AttributeDefinition{
			{
				AttributeName: aws.String("email"),
				AttributeType: dbtypes.ScalarAttributeTypeS,
			},
		},
		KeySchema: []dbtypes.KeySchemaElement{
			{
				AttributeName: aws.String("email"),
				KeyType:       dbtypes.KeyTypeHash,
			},
		},
		BillingMode: dbtypes.BillingModePayPerRequest,
	})
	must.NoError(t, err, must.Sprint("failed to create test table"))

	t.Cleanup(func() {
		_, err = d.DeleteTable(ctx, &dynamodb.DeleteTableInput{TableName: tableName})
		if err != nil {
			t.Logf("failed to delete table: %v", err)
		}
	})

	// Insert a test item
	pbsValue := structpb.NewStructValue(&structpb.Struct{
		Fields: map[string]*structpb.Value{
			"email": structpb.NewStringValue("test@test.com"),
			"name":  structpb.NewStringValue("Test User"),
			"roles": structpb.NewStructValue(&structpb.Struct{
				Fields: map[string]*structpb.Value{
					"admin": structpb.NewStringValue(timestamppb.Now().String()),
				},
			}),
		},
	})
	item, err := dynabuf.Marshal(pbsValue)
	must.NoError(t, err, must.Sprint("failed to marshal protobuf message to dynamodb item"))

	_, err = d.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: tableName,
		Item:      item.(map[string]dbtypes.AttributeValue),
	})
	must.NoError(t, err, must.Sprint("failed to put item"))

	out, err := d.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: tableName,
		Key: map[string]dbtypes.AttributeValue{
			"email": &dbtypes.AttributeValueMemberS{Value: "test@test.com"},
		},
	})
	must.NoError(t, err, must.Sprint("failed to get item"))
	must.MapContainsKeys(t, out.Item, []string{"email", "name", "roles"})

	env, err := expr.NewEnv(expr.MessageFieldVariables(pbsValue)...)
	must.NoError(t, err, must.Sprint("failed to create CEL env with message fields"))

	keyCondAst, issues := env.Compile(`email == "test@test.com"`)
	must.NoError(t, issues.Err(), must.Sprint("compilation error for key condition"))

	keyCond, err := expr.KeyCondition(keyCondAst)
	must.NoError(t, err, must.Sprint("failed to create key condition expression"))

	isAdmin, issues := env.Compile(`has(roles.admin)`)
	must.NoError(t, issues.Err(), must.Sprint("compilation error for filter"))

	cond, err := expr.Filter(isAdmin)
	must.NoError(t, err, must.Sprint("failed to create filter expression"))

	queryExpr, err := expression.NewBuilder().WithKeyCondition(keyCond).WithFilter(cond).Build()
	must.NoError(t, err, must.Sprint("failed to build key cond with filter expression"))

	out2, err := d.Query(ctx, &dynamodb.QueryInput{
		TableName:                 tableName,
		KeyConditionExpression:    queryExpr.KeyCondition(),
		FilterExpression:          queryExpr.Filter(),
		ExpressionAttributeNames:  queryExpr.Names(),
		ExpressionAttributeValues: queryExpr.Values(),
	})
	must.NoError(t, err, must.Sprint("query failed"))
	must.Len(t, 1, out2.Items)
	must.MapContainsKeys(t, out2.Items[0], []string{"email", "name", "roles"})

	scanFilterAST, issues := env.Compile(`name == "Test User"`)
	must.NoError(t, issues.Err(), must.Sprint("compilation error for filter"))

	scanFilterCond, err := expr.Filter(scanFilterAST)
	must.NoError(t, err, must.Sprint("failed to create filter expression"))

	scanProjAst, issues := env.Compile(`["email", "name"]`)
	must.NoError(t, issues.Err(), must.Sprint("compilation error for projection"))

	scanProj, err := expr.Projection(scanProjAst)
	must.NoError(t, err, must.Sprint("failed to create projection expression"))

	scanExpr, err := expression.NewBuilder().WithFilter(scanFilterCond).WithProjection(scanProj).Build()
	must.NoError(t, err, must.Sprint("failed to build filter expression"))

	out3, err := d.Scan(ctx, &dynamodb.ScanInput{
		TableName:                 tableName,
		FilterExpression:          scanExpr.Filter(),
		ExpressionAttributeNames:  scanExpr.Names(),
		ExpressionAttributeValues: scanExpr.Values(),
		ProjectionExpression:      scanExpr.Projection(),
	})
	must.NoError(t, err, must.Sprint("scan failed"))
	must.Len(t, 1, out3.Items)
	must.MapContainsKeys(t, out3.Items[0], []string{"email", "name"})
	must.MapNotContainsKey(t, out3.Items[0], "roles")
}
