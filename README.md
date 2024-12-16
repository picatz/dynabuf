# dynabuf
 
**Dynabuf** converts [Protocol Buffers] to [attribute maps] for [DynamoDB], 
and back again.
 
[Protocol Buffers]: https://developers.google.com/protocol-buffers
[DynamoDB]: https://aws.amazon.com/dynamodb/
[attribute maps]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_AttributeValue.html

## Installation

```console
$ go get -v github.com/picatz/dynabuf@latest
```

## How Does It Work?

Dynabuf leverages [`protojson`] as an intermediate format to convert between Protocol Buffers 
and DynamoDB attribute maps. This approach utilizes the [standard JSON mapping] provided by 
Protocol Buffers, ensuring a consistent and predictable conversion process. By using Dynabuf, 
you maintain the ability to perform DynamoDB queries on your data structures without sacrificing 
type safety or requiring additional boilerplate code. No manual field mapping or [`protoc`] plugins required.

> [!IMPORTANT]
>
> This library is designed to be used with the [AWS SDK for Go] (v2).

[`protojson`]: https://pkg.go.dev/google.golang.org/protobuf/encoding/protojson
[`protoc`]: https://protobuf.dev/reference/other/#plugins
[standard JSON mapping]: https://protobuf.dev/programming-guides/proto3/#json
[AWS SDK for Go]: https://aws.amazon.com/sdk-for-go/

## Usage

The `dynabuf` package provides two main functions: `Marshal` and `Unmarshal` that
should feel familiar to most Go developers. The `expr` package provides a way to
build DynamoDB expressions using [Common Expression Language] (CEL) syntax.

[Common Expression Language]: https://cel.dev

### Marshal and Unmarshal

You define a Protocol Buffer message, generate the corresponding Go struct,
and then use the [`Marshal`] and [`Unmarshal`] functions to convert 
between the Protocol Buffer message and a DynamoDB [attribute value],
such as an [attribute map].

[generate]: https://buf.build/docs/reference/cli/buf/generate
[`Marshal`]: https://pkg.go.dev/github.com/picatz/dynabuf#Marshal
[`Unmarshal`]: https://pkg.go.dev/github.com/picatz/dynabuf#Unmarshal
[attribute value]: https://docs.aws.amazon.com/amazondynamodb/latest/APIReference/API_AttributeValue.html
[attribute map]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue#MarshalMap

```protobuf
syntax = "proto3";

package example;

message User {
  string id = 1;
  string name = 2;
  string email = 3;
}
```

```go
resp, _ := dynamoClient.CreateTable(
	ctx,
	&dynamodb.CreateTableInput{
		AttributeDefinitions: []types.AttributeDefinition{
			{
				AttributeName: aws.String("id"),
				AttributeType: types.ScalarAttributeTypeS,
			},
		},
		KeySchema: []types.KeySchemaElement{
			{
				AttributeName: aws.String("id"),
				KeyType:       types.KeyTypeHash,
			},
		},
		TableName:   aws.String(table),
		BillingMode: types.BillingModePayPerRequest,
	},
)

user := &example.User{
    Id: "123",
    Name: "John Doe",
    Email: "...",
}

item, _ := dynabuf.Marshal(user)

dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
	TableName: tableName,
	Item:      item,
})

output, _ := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
	TableName: tableName,
	Key: map[string]types.AttributeValue{
		"id": &types.AttributeValueMemberS{Value: user.Id},
	},
})

outputProto := &example.User{}
dynabuf.Unmarshal(output.Item, outputProto)

fmt.Println(outputProto)
// &example.User{Id: "123", Name: "John Doe", Email: "..."}
```

> [!TIP]
>
> The `Marshal` and `Unmarshal` functions can be used to convert single
> messages or a slice of messages. This is particularly useful when working
> with batch operations in DynamoDB, where you may need to convert multiple
> attribute maps at once, without the extra loop logic.

### Expressions

You can use the [`expr`] package to build DynamoDB [`expression`]s for use with
DynamoDB operations like `Query`, `Scan`, `UpdateItem`, and `DeleteItem`.

[`expr`]: https://pkg.go.dev/github.com/picatz/dynabuf/expr
[`expression`]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression

```protobuf
syntax = "proto3";

package example;

message UserOrder {
  string user = 1;
  string order = 2;
  string status = 3;
  repeated string tags = 4;
}
```

```go
userOrder := &example.UserOrder{
	User:  "123",
	Order: "456",
	Status: "pending",
	Tags:  []string{"urgent"},
}

item, _ := dynabuf.Marshal(userOrder)

dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
	TableName: tableName,
	Item:      item,
})

env, _ := expr.NewEnv(
	expr.MessageFieldVariables(userOrder)...,
	// cel.Variable("user", types.StringType),  // partition key
	// cel.Variable("order", types.StringType), // sort key
	// cel.Variable("status", types.StringType),
	// cel.Variable("tags", types.NewListType(types.StringType)),
)

keyAst, _ := env.Compile(`user == "123" && order == "456"`)
keyCond, _ := expr.KeyCondition(keyAst)

filterAst, _ := env.Compile(`status == "pending" && "urgent" in tags`)
filterCond, _ := expr.Filter(filterAst)

keyCondAndFilterExpr, _ := expression.NewBuilder().WithKeyCondition(keyCond).WithFilter(filterCond).Build()

queryInput := dynamodb.QueryInput{
	TableName:                 aws.String("example-table"),
	KeyConditionExpression:    keyCondAndFilterExpr.KeyCondition(),
	FilterExpression:          keyCondAndFilterExpr.Filter(),
	ExpressionAttributeNames:  keyCondAndFilterExpr.Names(),
	ExpressionAttributeValues: keyCondAndFilterExpr.Values(),
}
```