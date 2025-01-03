package expr

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/expression"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/decls"
	"github.com/google/cel-go/common/operators"
	"github.com/google/cel-go/common/overloads"
	celtypes "github.com/google/cel-go/common/types"
	"github.com/google/cel-go/common/types/ref"
	exprpb "google.golang.org/genproto/googleapis/api/expr/v1alpha1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

// NewEnv returns a new CEL environment configured for DynamoDB expressions.
func NewEnv(opts ...cel.EnvOption) (*cel.Env, error) {
	opts = append(opts, macros...)
	opts = append(opts, dynamoDBFunctions...)
	opts = append(opts, updateFunctions...)

	env, err := cel.NewEnv(opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}
	return env, nil
}

// macros defines the CEL macros supported. Here we clear existing macros and support only `has`.
var macros = []cel.EnvOption{
	cel.ClearMacros(),
	cel.Macros(
		cel.HasMacro,
	),
}

// updateFunctions defines functions allowed in update expressions.
var updateFunctions = []cel.EnvOption{
	cel.Function("set",
		cel.Overload("set_overload",
			[]*cel.Type{celtypes.StringType, celtypes.DynType},
			celtypes.BoolType,
		),
	),
	cel.Function("remove",
		cel.Overload("remove_overload",
			[]*cel.Type{celtypes.StringType},
			celtypes.BoolType,
		),
	),
	cel.Function("add",
		cel.Overload("add_overload",
			[]*cel.Type{celtypes.StringType, celtypes.DynType},
			celtypes.BoolType,
		),
	),
	cel.Function("delete",
		cel.Overload("delete_overload",
			[]*cel.Type{celtypes.StringType, celtypes.DynType},
			celtypes.BoolType,
		),
	),
}

// dynamoDBFunctions defines functions corresponding to DynamoDB condition functions.
//
// These are fake functions that are not implemented, which is intentional. The functions are
// placeholders to allow the CEL expressions to be parsed and checked. The actual conversion
// of the expressions to DynamoDB expressions is done by the converter, and evaluation is done
// by DynamoDB, not by CEL.
var dynamoDBFunctions = []cel.EnvOption{
	cel.Function("attribute_exists", cel.Overload(
		"attribute_exists",
		[]*celtypes.Type{celtypes.DynType},
		celtypes.BoolType,
		decls.FunctionBinding(func(values ...ref.Val) ref.Val {
			return celtypes.NewErr("attribute_exists function not implemented")
		}),
	)),
	cel.Function("attribute_not_exists", cel.Overload(
		"attribute_not_exists",
		[]*celtypes.Type{celtypes.DynType},
		celtypes.BoolType,
		decls.FunctionBinding(func(values ...ref.Val) ref.Val {
			return celtypes.NewErr("attribute_not_exists function not implemented")
		}),
	)),
	cel.Function("begins_with", cel.Overload(
		"begins_with",
		[]*celtypes.Type{celtypes.StringType, celtypes.StringType},
		celtypes.BoolType,
		decls.FunctionBinding(func(values ...ref.Val) ref.Val {
			return celtypes.NewErr("begins_with function not implemented")
		}),
	)),
	cel.Function("attribute_type", cel.Overload(
		"attribute_type",
		[]*celtypes.Type{celtypes.DynType, celtypes.StringType},
		celtypes.BoolType,
		decls.FunctionBinding(func(values ...ref.Val) ref.Val {
			return celtypes.NewErr("attribute_type function not implemented")
		}),
	)),
	cel.Function(
		"contains",
		cel.Overload(
			"contains_list",
			[]*celtypes.Type{celtypes.NewListType(celtypes.StringType), celtypes.StringType},
			celtypes.BoolType,
			decls.FunctionBinding(func(values ...ref.Val) ref.Val {
				return celtypes.NewErr("contains function not implemented")
			}),
		),
		cel.Overload(
			"contains_string_string",
			[]*celtypes.Type{celtypes.StringType, celtypes.StringType},
			celtypes.BoolType,
			decls.FunctionBinding(func(values ...ref.Val) ref.Val {
				return celtypes.NewErr("contains function not implemented")
			}),
		),
	),
}

// ConditionExpression converts a CEL AST into a DynamoDB condition expression.
func Condition(ast *cel.Ast) (expression.ConditionBuilder, error) {
	return newConverter().Condition(ast)
}

// Filter converts a CEL AST into a DynamoDB filter expression.
func Filter(ast *cel.Ast) (expression.ConditionBuilder, error) {
	return newConverter().Filter(ast)
}

// KeyCondition converts a CEL AST into a DynamoDB key condition expression.
func KeyCondition(ast *cel.Ast) (expression.KeyConditionBuilder, error) {
	return newConverter().KeyCondition(ast)
}

// Projection converts a CEL AST into a DynamoDB projection expression.
func Projection(ast *cel.Ast) (expression.ProjectionBuilder, error) {
	return newConverter().Projection(ast)
}

// Update converts a CEL AST into a DynamoDB update expression.
func Update(ast *cel.Ast) (expression.UpdateBuilder, error) {
	return newConverter().Update(ast)
}

type converter struct {
	typeMap map[int64]*exprpb.Type
}

func newConverter() *converter {
	return &converter{}
}

func (c *converter) Condition(ast *cel.Ast) (expression.ConditionBuilder, error) {
	checkedExpr, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("failed to convert AST to checked expression: %w", err)
	}
	c.typeMap = checkedExpr.TypeMap

	cond, err := c.conditionFromExpr(checkedExpr.Expr)
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("failed to convert condition expression: %w", err)
	}

	return cond, nil
}

func (c *converter) Filter(ast *cel.Ast) (expression.ConditionBuilder, error) {
	checkedExpr, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("failed to convert AST to checked expression: %w", err)
	}
	c.typeMap = checkedExpr.TypeMap

	cond, err := c.conditionFromExpr(checkedExpr.Expr)
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("failed to convert filter expression: %w", err)
	}

	return cond, nil
}

func (c *converter) KeyCondition(ast *cel.Ast) (expression.KeyConditionBuilder, error) {
	checkedExpr, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return expression.KeyConditionBuilder{}, fmt.Errorf("failed to convert AST to checked expression: %w", err)
	}
	c.typeMap = checkedExpr.TypeMap

	keyCond, err := c.keyConditionFromExpr(checkedExpr.Expr)
	if err != nil {
		return expression.KeyConditionBuilder{}, fmt.Errorf("failed to convert key condition expression: %w", err)
	}

	return keyCond, nil
}

func (c *converter) Projection(ast *cel.Ast) (expression.ProjectionBuilder, error) {
	checkedExpr, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return expression.ProjectionBuilder{}, fmt.Errorf("failed to convert AST to checked expression: %w", err)
	}
	c.typeMap = checkedExpr.TypeMap

	proj, err := c.projectionFromExpr(checkedExpr.Expr)
	if err != nil {
		return expression.ProjectionBuilder{}, fmt.Errorf("failed to convert projection expression: %w", err)
	}

	return proj, nil
}

func (c *converter) Update(ast *cel.Ast) (expression.UpdateBuilder, error) {
	checkedExpr, err := cel.AstToCheckedExpr(ast)
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("failed to convert AST to checked expression: %w", err)
	}
	c.typeMap = checkedExpr.TypeMap

	update, err := c.updateFromExpr(checkedExpr.Expr)
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("failed to convert update expression: %w", err)
	}

	return update, nil
}

func (c *converter) conditionFromExpr(expr *exprpb.Expr) (expression.ConditionBuilder, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_CallExpr:
		callExpr := expr.GetCallExpr()
		if callExpr.Target != nil {
			// Macro calls like `has(x)`.
			return c.conditionFromMacro(expr)
		}
		return c.conditionFromCallExpr(expr)
	case *exprpb.Expr_ConstExpr:
		return c.conditionFromConstExpr(expr)
	case *exprpb.Expr_IdentExpr, *exprpb.Expr_SelectExpr:
		// For a bare identifier or field, treat as attribute_exists(name).
		operand, err := c.operandFromExpr(expr)
		if err != nil {
			return expression.ConditionBuilder{}, err
		}
		nameBuilder, ok := operand.(expression.NameBuilder)
		if !ok {
			return expression.ConditionBuilder{}, fmt.Errorf("operand is not an attribute name in condition")
		}
		return expression.AttributeExists(nameBuilder), nil
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported expression kind in condition: %T", expr.ExprKind)
	}
}

func (c *converter) conditionFromMacro(expr *exprpb.Expr) (expression.ConditionBuilder, error) {
	callExpr := expr.GetCallExpr()
	targetExpr := callExpr.Target
	targetName, err := c.getAttributeName(targetExpr)
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("failed to get target name for macro: %w", err)
	}

	nameBuilder := expression.Name(targetName)
	switch callExpr.Function {
	case "has":
		return expression.AttributeExists(nameBuilder), nil
	case "startsWith":
		return expression.BeginsWith(nameBuilder, callExpr.Args[0].GetConstExpr().GetStringValue()), nil
	case "contains":
		return expression.Contains(nameBuilder, callExpr.Args[0].GetConstExpr().GetStringValue()), nil
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported macro in condition: %q", callExpr.Function)
	}
}

func (c *converter) conditionFromConstExpr(expr *exprpb.Expr) (expression.ConditionBuilder, error) {
	constExpr := expr.GetConstExpr()
	if constExpr == nil {
		return expression.ConditionBuilder{}, errors.New("constant expression is nil")
	}

	switch val := constExpr.ConstantKind.(type) {
	case *exprpb.Constant_BoolValue:
		if val.BoolValue {
			// Always true - DynamoDB does not have a direct "true" condition. Return a no-op condition.
			// In some contexts, you may omit the condition if always true is desired.
			// Here, return a no-op (AttributeExists on a guaranteed-existing attribute?), or just no condition.
			// We'll return empty ConditionBuilder which means no condition set. This may or may not be desired.
			return expression.ConditionBuilder{}, nil
		}
		// Always false is not directly representable as a condition. Return error.
		return expression.ConditionBuilder{}, fmt.Errorf("condition that is always false is not supported")
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported constant type in condition: %T", val)
	}
}

func (c *converter) conditionFromCallExpr(expr *exprpb.Expr) (expression.ConditionBuilder, error) {
	callExpr := expr.GetCallExpr()
	if callExpr == nil {
		return expression.ConditionBuilder{}, errors.New("call expression is nil")
	}

	switch callExpr.Function {
	case operators.LogicalAnd:
		return c.conditionFromLogicalAnd(callExpr)
	case operators.LogicalOr:
		return c.conditionFromLogicalOr(callExpr)
	case operators.LogicalNot:
		return c.conditionFromLogicalNot(callExpr)
	case operators.Equals, operators.NotEquals, operators.Greater, operators.GreaterEquals, operators.Less, operators.LessEquals:
		return c.conditionFromComparison(callExpr)
	case operators.In:
		return c.conditionFromIn(callExpr)
	case overloads.Contains:
		return c.conditionFromContains(callExpr)
	case "attribute_exists":
		return c.conditionFromAttributeExists(callExpr)
	case "attribute_not_exists":
		return c.conditionFromAttributeNotExists(callExpr)
	case "begins_with":
		return c.conditionFromBeginsWith(callExpr)
	case "attribute_type":
		return c.conditionFromAttributeType(callExpr)
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported function in condition: %q", callExpr.Function)
	}
}

func (c *converter) conditionFromLogicalAnd(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("AND operator: %w", err)
	}

	leftCond, err := c.conditionFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("AND operator left operand: %w", err)
	}

	rightCond, err := c.conditionFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("AND operator right operand: %w", err)
	}

	return expression.And(leftCond, rightCond), nil
}

func (c *converter) conditionFromLogicalOr(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("OR operator: %w", err)
	}

	leftCond, err := c.conditionFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("OR operator left operand: %w", err)
	}

	rightCond, err := c.conditionFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("OR operator right operand: %w", err)
	}

	return expression.Or(leftCond, rightCond), nil
}

func (c *converter) conditionFromLogicalNot(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 1); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("NOT operator: %w", err)
	}

	cond, err := c.conditionFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("NOT operator operand: %w", err)
	}

	return expression.Not(cond), nil
}

func (c *converter) conditionFromComparison(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("comparison operator: %w", err)
	}

	leftOperand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("comparison operator left operand: %w", err)
	}

	rightOperand, err := c.operandFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("comparison operator right operand: %w", err)
	}

	switch callExpr.Function {
	case operators.Equals:
		return expression.Equal(leftOperand, rightOperand), nil
	case operators.NotEquals:
		return expression.NotEqual(leftOperand, rightOperand), nil
	case operators.Greater:
		return expression.GreaterThan(leftOperand, rightOperand), nil
	case operators.GreaterEquals:
		return expression.GreaterThanEqual(leftOperand, rightOperand), nil
	case operators.Less:
		return expression.LessThan(leftOperand, rightOperand), nil
	case operators.LessEquals:
		return expression.LessThanEqual(leftOperand, rightOperand), nil
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported comparison operator: %q", callExpr.Function)
	}
}

func (c *converter) conditionFromIn(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("IN operator: %w", err)
	}

	leftOperand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("IN operator left operand: %w", err)
	}

	operands, err := c.getListOfOperandsFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("IN operator right operand: %w", err)
	}

	if len(operands) == 0 {
		return expression.ConditionBuilder{}, errors.New("IN operator requires a non-empty list of values")
	}

	return expression.In(leftOperand, operands[0], operands[1:]...), nil
}

func (c *converter) conditionFromContains(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("contains function: %w", err)
	}

	nameOperand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("contains function first arg: %w", err)
	}

	nameBuilder, ok := nameOperand.(expression.NameBuilder)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("contains function first argument must be an attribute name")
	}

	value, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("contains function second arg: %w", err)
	}

	return expression.Contains(nameBuilder, value), nil
}

func (c *converter) conditionFromAttributeExists(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 1); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_exists function: %w", err)
	}

	operand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_exists argument: %w", err)
	}

	nameBuilder, ok := operand.(expression.NameBuilder)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("attribute_exists arg must be an attribute name")
	}

	return expression.AttributeExists(nameBuilder), nil
}

func (c *converter) conditionFromAttributeNotExists(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 1); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_not_exists function: %w", err)
	}

	operand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_not_exists argument: %w", err)
	}

	nameBuilder, ok := operand.(expression.NameBuilder)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("attribute_not_exists arg must be an attribute name")
	}

	return expression.AttributeNotExists(nameBuilder), nil
}

func (c *converter) conditionFromBeginsWith(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("begins_with function: %w", err)
	}

	nameOperand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("begins_with first arg: %w", err)
	}

	nameBuilder, ok := nameOperand.(expression.NameBuilder)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("begins_with first argument must be attribute name")
	}

	value, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("begins_with second arg: %w", err)
	}

	strValue, ok := value.(string)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("begins_with second arg must be a string")
	}

	return expression.BeginsWith(nameBuilder, strValue), nil
}

func (c *converter) conditionFromAttributeType(callExpr *exprpb.Expr_Call) (expression.ConditionBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_type function: %w", err)
	}

	operand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_type first arg: %w", err)
	}

	nameBuilder, ok := operand.(expression.NameBuilder)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("attribute_type first argument must be attribute name")
	}

	typeValue, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.ConditionBuilder{}, fmt.Errorf("attribute_type second arg: %w", err)
	}

	typeStr, ok := typeValue.(string)
	if !ok {
		return expression.ConditionBuilder{}, errors.New("attribute_type second arg must be a string")
	}

	var exprType expression.DynamoDBAttributeType
	switch typeStr {
	case "S":
		exprType = expression.String
	case "SS":
		exprType = expression.StringSet
	case "N":
		exprType = expression.Number
	case "B":
		exprType = expression.Binary
	case "BOOL":
		exprType = expression.Boolean
	case "NULL":
		exprType = expression.Null
	case "M":
		exprType = expression.Map
	case "L":
		exprType = expression.List
	default:
		return expression.ConditionBuilder{}, fmt.Errorf("unsupported attribute type: %q", typeStr)
	}

	return expression.AttributeType(nameBuilder, exprType), nil
}

func (c *converter) keyConditionFromExpr(expr *exprpb.Expr) (expression.KeyConditionBuilder, error) {
	// Key conditions are more restricted. We expect call expressions.
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_CallExpr:
		return c.keyConditionFromCallExpr(expr)
	default:
		return expression.KeyConditionBuilder{}, fmt.Errorf("unsupported expression kind in key condition: %T", expr.ExprKind)
	}
}

func (c *converter) keyConditionFromCallExpr(expr *exprpb.Expr) (expression.KeyConditionBuilder, error) {
	callExpr := expr.GetCallExpr()
	if callExpr == nil {
		return expression.KeyConditionBuilder{}, errors.New("key condition call expression is nil")
	}

	// Special handling for AND conditions in key conditions.
	if callExpr.Function == operators.LogicalAnd {
		if err := c.expectArgCount(callExpr, 2); err != nil {
			return expression.KeyConditionBuilder{}, fmt.Errorf("key condition AND: %w", err)
		}
		leftCond, err := c.keyConditionFromExpr(callExpr.Args[0])
		if err != nil {
			return expression.KeyConditionBuilder{}, fmt.Errorf("key condition AND left: %w", err)
		}
		rightCond, err := c.keyConditionFromExpr(callExpr.Args[1])
		if err != nil {
			return expression.KeyConditionBuilder{}, fmt.Errorf("key condition AND right: %w", err)
		}
		return expression.KeyAnd(leftCond, rightCond), nil
	}

	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.KeyConditionBuilder{}, fmt.Errorf("key condition operator %q: %w", callExpr.Function, err)
	}

	leftKey, err := c.getAttributeName(callExpr.Args[0])
	if err != nil {
		return expression.KeyConditionBuilder{}, fmt.Errorf("failed to get left key name: %w", err)
	}

	rightValue, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.KeyConditionBuilder{}, fmt.Errorf("failed to get right key value: %w", err)
	}

	keyBuilder := expression.Key(leftKey)
	valueBuilder := expression.Value(rightValue)

	switch callExpr.Function {
	case operators.Equals:
		return keyBuilder.Equal(valueBuilder), nil
	case operators.Greater:
		return keyBuilder.GreaterThan(valueBuilder), nil
	case operators.GreaterEquals:
		return keyBuilder.GreaterThanEqual(valueBuilder), nil
	case operators.Less:
		return keyBuilder.LessThan(valueBuilder), nil
	case operators.LessEquals:
		return keyBuilder.LessThanEqual(valueBuilder), nil
	case "begins_with":
		strVal, ok := rightValue.(string)
		if !ok {
			return expression.KeyConditionBuilder{}, errors.New("begins_with in key condition requires a string value")
		}
		return keyBuilder.BeginsWith(strVal), nil
	default:
		return expression.KeyConditionBuilder{}, fmt.Errorf("unsupported function in key condition: %q", callExpr.Function)
	}
}

func (c *converter) projectionFromExpr(expr *exprpb.Expr) (expression.ProjectionBuilder, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_ListExpr:
		return c.projectionFromListExpr(expr.GetListExpr())
	default:
		return expression.ProjectionBuilder{}, fmt.Errorf("unsupported expression kind in projection: %T", expr.ExprKind)
	}
}

func (c *converter) projectionFromListExpr(listExpr *exprpb.Expr_CreateList) (expression.ProjectionBuilder, error) {
	if listExpr == nil {
		return expression.ProjectionBuilder{}, errors.New("projection list expression is nil")
	}
	var names []expression.NameBuilder
	for _, elem := range listExpr.Elements {
		name, err := c.getAttributeName(elem)
		if err != nil {
			return expression.ProjectionBuilder{}, fmt.Errorf("projection element: %w", err)
		}
		names = append(names, expression.Name(name))
	}

	if len(names) == 0 {
		return expression.ProjectionBuilder{}, errors.New("projection list cannot be empty")
	}

	return expression.NamesList(names[0], names[1:]...), nil
}

func (c *converter) updateFromExpr(expr *exprpb.Expr) (expression.UpdateBuilder, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_CallExpr:
		return c.updateFromCallExpr(expr)
	default:
		return expression.UpdateBuilder{}, fmt.Errorf("unsupported expression kind in update: %T", expr.ExprKind)
	}
}

func (c *converter) updateFromCallExpr(expr *exprpb.Expr) (expression.UpdateBuilder, error) {
	callExpr := expr.GetCallExpr()
	if callExpr == nil {
		return expression.UpdateBuilder{}, errors.New("update call expression is nil")
	}

	switch callExpr.Function {
	case "set":
		return c.updateSetFromCallExpr(callExpr)
	case "remove":
		return c.updateRemoveFromCallExpr(callExpr)
	case "add":
		return c.updateAddFromCallExpr(callExpr)
	case "delete":
		return c.updateDeleteFromCallExpr(callExpr)
	default:
		return expression.UpdateBuilder{}, fmt.Errorf("unsupported update function: %q", callExpr.Function)
	}
}

func (c *converter) updateSetFromCallExpr(callExpr *exprpb.Expr_Call) (expression.UpdateBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("set function: %w", err)
	}

	fieldName, err := c.getStringValue(callExpr.Args[0])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("set function field name: %w", err)
	}

	valueOperand, err := c.operandFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("set function value: %w", err)
	}

	return expression.Set(expression.Name(fieldName), valueOperand), nil
}

func (c *converter) updateRemoveFromCallExpr(callExpr *exprpb.Expr_Call) (expression.UpdateBuilder, error) {
	if err := c.expectArgCount(callExpr, 1); err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("remove function: %w", err)
	}

	fieldName, err := c.getStringValue(callExpr.Args[0])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("remove function field name: %w", err)
	}

	return expression.Remove(expression.Name(fieldName)), nil
}

func (c *converter) updateAddFromCallExpr(callExpr *exprpb.Expr_Call) (expression.UpdateBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("add function: %w", err)
	}

	fieldName, err := c.getStringValue(callExpr.Args[0])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("add function field name: %w", err)
	}

	valueOperand, err := c.operandFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("add function value: %w", err)
	}

	valueBuilder, ok := valueOperand.(expression.ValueBuilder)
	if !ok {
		return expression.UpdateBuilder{}, errors.New("add function value must be a ValueBuilder")
	}

	return expression.Add(expression.Name(fieldName), valueBuilder), nil
}

func (c *converter) updateDeleteFromCallExpr(callExpr *exprpb.Expr_Call) (expression.UpdateBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("delete function: %w", err)
	}

	fieldName, err := c.getStringValue(callExpr.Args[0])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("delete function field name: %w", err)
	}

	valueOperand, err := c.operandFromExpr(callExpr.Args[1])
	if err != nil {
		return expression.UpdateBuilder{}, fmt.Errorf("delete function value: %w", err)
	}

	valueBuilder, ok := valueOperand.(expression.ValueBuilder)
	if !ok {
		return expression.UpdateBuilder{}, errors.New("delete function value must be a ValueBuilder")
	}

	return expression.Delete(expression.Name(fieldName), valueBuilder), nil
}

func (c *converter) operandFromExpr(expr *exprpb.Expr) (expression.OperandBuilder, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_ConstExpr:
		return c.operandFromConstExpr(expr)
	case *exprpb.Expr_IdentExpr:
		name, err := c.getAttributeName(expr)
		if err != nil {
			return nil, fmt.Errorf("operand ident: %w", err)
		}
		return expression.Name(name), nil
	case *exprpb.Expr_SelectExpr:
		name, err := c.getAttributeName(expr)
		if err != nil {
			return nil, fmt.Errorf("operand select: %w", err)
		}
		return expression.Name(name), nil
	case *exprpb.Expr_CallExpr:
		return c.operandFromCallExprOperand(expr)
	case *exprpb.Expr_ListExpr:
		return c.operandFromListExpr(expr.GetListExpr())
	default:
		return nil, fmt.Errorf("unsupported expression kind in operand: %T", expr.ExprKind)
	}
}

func (c *converter) operandFromConstExpr(expr *exprpb.Expr) (expression.OperandBuilder, error) {
	val, err := c.getValueFromExpr(expr)
	if err != nil {
		return nil, fmt.Errorf("operand const: %w", err)
	}
	return expression.Value(val), nil
}

func (c *converter) operandFromListExpr(listExpr *exprpb.Expr_CreateList) (expression.OperandBuilder, error) {
	if listExpr == nil {
		return nil, errors.New("list expression is nil")
	}
	var values []any
	for i, elem := range listExpr.Elements {
		val, err := c.getValueFromExpr(elem)
		if err != nil {
			return nil, fmt.Errorf("failed to get value for list element at index %d: %w", i, err)
		}
		values = append(values, val)
	}
	return expression.Value(values), nil
}

func (c *converter) operandFromCallExprOperand(expr *exprpb.Expr) (expression.OperandBuilder, error) {
	callExpr := expr.GetCallExpr()
	switch callExpr.Function {
	case overloads.Size:
		return c.operandFromSizeFunction(callExpr)
	case operators.Index:
		return c.operandFromIndexOperator(callExpr)
	default:
		return nil, fmt.Errorf("unsupported function call in operand: %q", callExpr.Function)
	}
}

func (c *converter) operandFromSizeFunction(callExpr *exprpb.Expr_Call) (expression.OperandBuilder, error) {
	if err := c.expectArgCount(callExpr, 1); err != nil {
		return nil, fmt.Errorf("size function: %w", err)
	}

	operand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return nil, fmt.Errorf("size function operand: %w", err)
	}

	nameBuilder, ok := operand.(expression.NameBuilder)
	if !ok {
		return nil, errors.New("size function argument must be an attribute name")
	}

	return expression.Size(nameBuilder), nil
}

func (c *converter) operandFromIndexOperator(callExpr *exprpb.Expr_Call) (expression.OperandBuilder, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return nil, fmt.Errorf("index operator: %w", err)
	}

	collectionOperand, err := c.operandFromExpr(callExpr.Args[0])
	if err != nil {
		return nil, fmt.Errorf("index operator collection operand: %w", err)
	}

	collectionName, ok := collectionOperand.(expression.NameBuilder)
	if !ok {
		return nil, errors.New("index operator collection operand must be an attribute name")
	}

	indexValue, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return nil, fmt.Errorf("index operator index value: %w", err)
	}

	indexStr, ok := indexValue.(string)
	if !ok {
		return nil, errors.New("index operand must be a string key for attribute access")
	}

	return collectionName.AppendName(expression.Name(indexStr)), nil
}

func (c *converter) getValueFromExpr(expr *exprpb.Expr) (any, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_ConstExpr:
		return c.constValue(expr.GetConstExpr())
	case *exprpb.Expr_ListExpr:
		listExpr := expr.GetListExpr()
		if listExpr == nil {
			return nil, errors.New("list expression is nil")
		}
		var values []any
		for i, elem := range listExpr.Elements {
			val, err := c.getValueFromExpr(elem)
			if err != nil {
				return nil, fmt.Errorf("list element at %d: %w", i, err)
			}
			values = append(values, val)
		}
		return values, nil
	case *exprpb.Expr_CallExpr:
		callExpr := expr.GetCallExpr()
		switch callExpr.Function {
		case operators.Index:
			// If we want to handle this as a value, attempt to resolve its operand.
			// Typically index operators return attribute paths, not raw values.
			// We'll return an error here since values should not be derived from attribute paths.
			return nil, errors.New("cannot directly get a value from an index operator without context")
		case "begins_with":
			// For begins_with, the second argument is the string prefix.
			// Caller should handle properly. We can just try extracting:
			return c.getValueFromExpr(callExpr.Args[1])
		default:
			return nil, fmt.Errorf("unsupported function call in value extraction: %q", callExpr.Function)
		}
	case *exprpb.Expr_IdentExpr:
		name, err := c.getAttributeName(expr)
		if err != nil {
			return nil, err
		}
		// Identifiers by themselves represent attribute names. If needed as a value, return the name.
		return name, nil
	default:
		return nil, fmt.Errorf("unsupported expression kind in value extraction: %T", expr.ExprKind)
	}
}

func (c *converter) constValue(constExpr *exprpb.Constant) (any, error) {
	if constExpr == nil {
		return nil, errors.New("constant expression is nil")
	}
	switch val := constExpr.ConstantKind.(type) {
	case *exprpb.Constant_BoolValue:
		return val.BoolValue, nil
	case *exprpb.Constant_Int64Value:
		return val.Int64Value, nil
	case *exprpb.Constant_Uint64Value:
		return val.Uint64Value, nil
	case *exprpb.Constant_DoubleValue:
		return val.DoubleValue, nil
	case *exprpb.Constant_StringValue:
		return val.StringValue, nil
	case *exprpb.Constant_BytesValue:
		return val.BytesValue, nil
	case *exprpb.Constant_NullValue:
		return nil, nil
	default:
		return nil, fmt.Errorf("unsupported constant kind: %T", val)
	}
}

func (c *converter) getAttributeName(expr *exprpb.Expr) (string, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_IdentExpr:
		return expr.GetIdentExpr().Name, nil
	case *exprpb.Expr_SelectExpr:
		parentName, err := c.getAttributeName(expr.GetSelectExpr().Operand)
		if err != nil {
			return "", fmt.Errorf("select expr parent: %w", err)
		}
		return fmt.Sprintf("%s.%s", parentName, expr.GetSelectExpr().Field), nil
	case *exprpb.Expr_CallExpr:
		callExpr := expr.GetCallExpr()
		switch callExpr.Function {
		case operators.Index:
			return c.getAttributeNameFromIndexCall(callExpr)
		case operators.GreaterEquals, operators.Equals:
			// Handle attribute names if they appear on left side of comparison.
			return c.getAttributeName(callExpr.Args[0])
		default:
			return "", fmt.Errorf("unsupported call expr in attribute name: %q", callExpr.Function)
		}
	case *exprpb.Expr_ConstExpr:
		// Allow constants as part of attribute name if needed.
		// Typically, attribute paths are strings. If the constant is a string, use it.
		val, err := c.constValue(expr.GetConstExpr())
		if err != nil {
			return "", fmt.Errorf("const in attribute name: %w", err)
		}
		strVal, ok := val.(string)
		if !ok {
			return "", fmt.Errorf("attribute name constant must be a string, got %T", val)
		}
		return strVal, nil
	default:
		return "", fmt.Errorf("unsupported expr kind in attribute name: %T", expr.ExprKind)
	}
}

func (c *converter) getAttributeNameFromIndexCall(callExpr *exprpb.Expr_Call) (string, error) {
	if err := c.expectArgCount(callExpr, 2); err != nil {
		return "", fmt.Errorf("index operator in attribute name: %w", err)
	}

	collectionName, err := c.getAttributeName(callExpr.Args[0])
	if err != nil {
		return "", fmt.Errorf("index operator collection name: %w", err)
	}

	indexValue, err := c.getValueFromExpr(callExpr.Args[1])
	if err != nil {
		return "", fmt.Errorf("index operator index value: %w", err)
	}

	indexStr, ok := indexValue.(string)
	if !ok {
		return "", errors.New("index operator index must be a string for attribute name")
	}

	return fmt.Sprintf("%s.%s", collectionName, indexStr), nil
}

func (c *converter) getListOfOperandsFromExpr(expr *exprpb.Expr) ([]expression.OperandBuilder, error) {
	switch expr.GetExprKind().(type) {
	case *exprpb.Expr_ListExpr:
		listExpr := expr.GetListExpr()
		var operands []expression.OperandBuilder
		for i, elem := range listExpr.Elements {
			operand, err := c.operandFromExpr(elem)
			if err != nil {
				return nil, fmt.Errorf("list operand at %d: %w", i, err)
			}
			operands = append(operands, operand)
		}
		return operands, nil
	case *exprpb.Expr_IdentExpr:
		// Single name as a list - treat as single operand list.
		name, err := c.getAttributeName(expr)
		if err != nil {
			return nil, err
		}
		return []expression.OperandBuilder{expression.Name(name)}, nil
	default:
		return nil, fmt.Errorf("unsupported kind in IN operator list: %T", expr.ExprKind)
	}
}

func (c *converter) getStringValue(expr *exprpb.Expr) (string, error) {
	value, err := c.getValueFromExpr(expr)
	if err != nil {
		return "", err
	}
	strVal, ok := value.(string)
	if !ok {
		return "", fmt.Errorf("expected string value, got %T", value)
	}
	return strVal, nil
}

func (c *converter) expectArgCount(callExpr *exprpb.Expr_Call, expected int) error {
	if len(callExpr.Args) != expected {
		return fmt.Errorf("expected %d arguments, got %d", expected, len(callExpr.Args))
	}
	return nil
}

// MessageVariable creates an environment option for using a whole proto message as a variable.
func MessageVariable(name string, msg proto.Message) []cel.EnvOption {
	typeName := msg.ProtoReflect().Descriptor().FullName()
	return []cel.EnvOption{
		cel.Types(msg),
		cel.Variable(name, celtypes.NewObjectType(string(typeName))),
	}
}

// MessageFieldVariables creates environment options for each field in a proto message.
// Fields are exposed as variables using their JSON names, because that's how they are
// stored in DynamoDB when coverted using protojson.
func MessageFieldVariables(msg proto.Message) []cel.EnvOption {
	var vars []cel.EnvOption
	switch msgTyped := msg.(type) {
	case *structpb.Value:
		// Handle google.protobuf.Struct via fieldsToEnvOptions
		if stVal, ok := msgTyped.Kind.(*structpb.Value_StructValue); ok {
			vars = append(vars, fieldsToEnvOptions(stVal.StructValue.Fields)...)
		} else {
			// Non-struct Values are just typed as-is
			vars = append(vars, cel.Types(msg))
		}
	case *structpb.Struct:
		for fieldName, value := range msgTyped.Fields {
			t := fieldTypeForStructValue(value)
			vars = append(vars, cel.Variable(fieldName, t))
		}
	default:
		// Handle non-structpb messages
		msgDescriptor := msg.ProtoReflect().Descriptor()
		vars = append(vars, cel.Types(msg))
		fields := msgDescriptor.Fields()
		for i := 0; i < fields.Len(); i++ {
			desc := fields.Get(i)
			t := fieldTypeForDescriptor(desc)
			fieldName := desc.JSONName()
			vars = append(vars, cel.Variable(fieldName, t))
		}
	}

	return vars
}

func fieldsToEnvOptions(fields map[string]*structpb.Value) []cel.EnvOption {
	var opts []cel.EnvOption
	for fieldName, value := range fields {
		t := fieldTypeForStructValue(value)
		opts = append(opts, cel.Variable(fieldName, t))
	}
	return opts
}

func fieldTypeForStructValue(value *structpb.Value) *celtypes.Type {
	switch value.Kind.(type) {
	case *structpb.Value_NullValue:
		return celtypes.NullType
	case *structpb.Value_NumberValue:
		return celtypes.DoubleType
	case *structpb.Value_StringValue:
		return celtypes.StringType
	case *structpb.Value_BoolValue:
		return celtypes.BoolType
	case *structpb.Value_StructValue:
		return celtypes.NewMapType(celtypes.StringType, celtypes.DynType)
	case *structpb.Value_ListValue:
		return celtypes.NewListType(celtypes.DynType)
	default:
		return celtypes.DynType
	}
}

func fieldTypeForDescriptor(desc protoreflect.FieldDescriptor) *celtypes.Type {
	switch desc.Kind() {
	case protoreflect.BoolKind:
		return celtypes.BoolType
	case protoreflect.Int32Kind, protoreflect.Sint32Kind, protoreflect.Sfixed32Kind,
		protoreflect.Int64Kind, protoreflect.Sint64Kind, protoreflect.Sfixed64Kind:
		return celtypes.IntType
	case protoreflect.Uint32Kind, protoreflect.Fixed32Kind, protoreflect.Uint64Kind, protoreflect.Fixed64Kind:
		return celtypes.UintType
	case protoreflect.FloatKind, protoreflect.DoubleKind:
		return celtypes.DoubleType
	case protoreflect.StringKind:
		return celtypes.StringType
	case protoreflect.BytesKind:
		return celtypes.BytesType
	case protoreflect.EnumKind:
		// Represent enums as strings
		return celtypes.StringType
	case protoreflect.MessageKind:
		fieldFullName := string(desc.Message().FullName())
		switch fieldFullName {
		case "google.protobuf.Timestamp":
			return celtypes.TimestampType
		case "google.protobuf.Duration":
			return celtypes.DurationType
		default:
			return celtypes.DynType
		}
	default:
		return celtypes.DynType
	}
}
