// Package expr provides a [transpiler] that converts [Common Expression Language] (CEL)
// expressions to Amazon [DynamoDB] expressions.
//
// The transpiler is implemented as a visitor that walks the CEL [Abstract Syntax Tree] (AST)
// and builds a [DynamoDB expression] for use in database operations. This package is
// designed to be used with the official AWS Go SDK v2 DynamoDB [expression] package to bridge
// the use of CEL and [Protocol Buffers] with DynamoDB.
//
// [transpiler]: https://en.wikipedia.org/wiki/Source-to-source_compiler
// [Common Expression Language]: https://cel.dev/
// [DynamoDB]: https://aws.amazon.com/dynamodb/
// [DynamoDB expression]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb/expression
// [Abstract Syntax Tree]: https://pkg.go.dev/github.com/google/cel-go/common/ast
// [expression]: https://pkg.go.dev/github.com/aws/aws-sdk-go-v2/service/dynamodb/expression
// [Protocol Buffers]: https://developers.google.com/protocol-buffers
package expr
