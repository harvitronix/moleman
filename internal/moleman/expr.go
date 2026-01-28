package moleman

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

func EvalCondition(expr string, data map[string]any) (bool, error) {
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "{{") && strings.HasSuffix(expr, "}}") {
		expr = strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(expr, "}}"), "{{"))
	}
	if expr == "" {
		return false, fmt.Errorf("empty condition")
	}

	node, err := parser.ParseExpr(expr)
	if err != nil {
		return false, fmt.Errorf("parse condition: %w", err)
	}
	value, err := evalExpr(node, data)
	if err != nil {
		return false, err
	}
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("condition did not evaluate to bool")
	}
	return b, nil
}

func evalExpr(node ast.Expr, data map[string]any) (any, error) {
	switch expr := node.(type) {
	case *ast.BasicLit:
		switch expr.Kind {
		case token.STRING:
			return strconv.Unquote(expr.Value)
		case token.INT:
			return strconv.Atoi(expr.Value)
		case token.FLOAT:
			return strconv.ParseFloat(expr.Value, 64)
		default:
			return nil, fmt.Errorf("unsupported literal")
		}
	case *ast.Ident:
		switch expr.Name {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return lookupPath(data, expr.Name)
		}
	case *ast.BinaryExpr:
		left, err := evalExpr(expr.X, data)
		if err != nil {
			return nil, err
		}
		right, err := evalExpr(expr.Y, data)
		if err != nil {
			return nil, err
		}
		return evalBinary(expr.Op, left, right)
	case *ast.ParenExpr:
		return evalExpr(expr.X, data)
	case *ast.SelectorExpr:
		base, err := evalExpr(expr.X, data)
		if err != nil {
			return nil, err
		}
		return lookupSelector(base, expr.Sel.Name)
	case *ast.IndexExpr:
		base, err := evalExpr(expr.X, data)
		if err != nil {
			return nil, err
		}
		indexValue, err := evalExpr(expr.Index, data)
		if err != nil {
			return nil, err
		}
		return lookupIndex(base, indexValue)
	default:
		return nil, fmt.Errorf("unsupported expression: %T", node)
	}
}

func evalBinary(op token.Token, left, right any) (any, error) {
	switch op {
	case token.LAND, token.LOR:
		lb, lok := left.(bool)
		rb, rok := right.(bool)
		if !lok || !rok {
			return nil, fmt.Errorf("logical ops require bools")
		}
		if op == token.LAND {
			return lb && rb, nil
		}
		return lb || rb, nil
	case token.EQL, token.NEQ, token.LSS, token.GTR, token.LEQ, token.GEQ:
		return compare(op, left, right)
	default:
		return nil, fmt.Errorf("unsupported operator: %s", op.String())
	}
}

func compare(op token.Token, left, right any) (bool, error) {
	switch l := left.(type) {
	case int:
		r, ok := coerceInt(right)
		if !ok {
			return false, fmt.Errorf("mismatched types for comparison")
		}
		return compareInts(op, l, r), nil
	case float64:
		r, ok := coerceFloat(right)
		if !ok {
			return false, fmt.Errorf("mismatched types for comparison")
		}
		return compareFloats(op, l, r), nil
	case string:
		rs, ok := right.(string)
		if !ok {
			return false, fmt.Errorf("mismatched types for comparison")
		}
		return compareStrings(op, l, rs), nil
	case bool:
		rb, ok := right.(bool)
		if !ok {
			return false, fmt.Errorf("mismatched types for comparison")
		}
		return compareBools(op, l, rb), nil
	default:
		return false, fmt.Errorf("unsupported comparison types")
	}
}

func compareInts(op token.Token, left, right int) bool {
	switch op {
	case token.EQL:
		return left == right
	case token.NEQ:
		return left != right
	case token.LSS:
		return left < right
	case token.GTR:
		return left > right
	case token.LEQ:
		return left <= right
	case token.GEQ:
		return left >= right
	default:
		return false
	}
}

func compareFloats(op token.Token, left, right float64) bool {
	switch op {
	case token.EQL:
		return left == right
	case token.NEQ:
		return left != right
	case token.LSS:
		return left < right
	case token.GTR:
		return left > right
	case token.LEQ:
		return left <= right
	case token.GEQ:
		return left >= right
	default:
		return false
	}
}

func compareStrings(op token.Token, left, right string) bool {
	switch op {
	case token.EQL:
		return left == right
	case token.NEQ:
		return left != right
	case token.LSS:
		return left < right
	case token.GTR:
		return left > right
	case token.LEQ:
		return left <= right
	case token.GEQ:
		return left >= right
	default:
		return false
	}
}

func compareBools(op token.Token, left, right bool) bool {
	switch op {
	case token.EQL:
		return left == right
	case token.NEQ:
		return left != right
	default:
		return false
	}
}

func coerceInt(value any) (int, bool) {
	switch v := value.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	default:
		return 0, false
	}
}

func coerceFloat(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func lookupPath(root map[string]any, path string) (any, error) {
	value, ok := root[path]
	if !ok {
		return nil, fmt.Errorf("unknown identifier: %s", path)
	}
	return value, nil
}

func lookupSelector(base any, key string) (any, error) {
	switch v := base.(type) {
	case map[string]any:
		value, ok := v[key]
		if !ok {
			return nil, fmt.Errorf("missing key: %s", key)
		}
		return value, nil
	default:
		return nil, fmt.Errorf("invalid selector on %T", base)
	}
}

func lookupIndex(base any, index any) (any, error) {
	switch v := base.(type) {
	case []map[string]any:
		i, ok := index.(int)
		if !ok {
			return nil, fmt.Errorf("index must be int")
		}
		if i < 0 || i >= len(v) {
			return nil, fmt.Errorf("index out of range")
		}
		return v[i], nil
	default:
		return nil, fmt.Errorf("invalid index on %T", base)
	}
}
