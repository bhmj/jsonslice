package jsonslice

import (
	"strconv"

	"github.com/bhmj/xpression"
)

// readFilter reads expression in ?( ... ) filter, parses tokens and writes result to nod.Filter.
// Consumes closing ) and ]
func readFilter(path []byte, i int, nod *tNode) (int, error) {
	e, err := findClosingBracket(path, i)
	if err != nil {
		return i, err
	}
	tokens, err := xpression.Parse(path[i:e])
	if err != nil {
		return i, err
	}
	nod.Filter = tokens
	nod.Type |= cFilter
	nod.Type &^= cDot

	l := len(path)
	if e < l {
		e++ // ')'
		if e < l && path[e] == ']' {
			e++
		}
	}
	return e, nil
}

// findClosingBracket returns the position of a closing round bracket (not consumed)
func findClosingBracket(path []byte, i int) (int, error) {
	var err error
	found := false
	l := len(path)
	count := 0
	for i < l {
		if path[i] == ')' && count == 0 {
			found = true
			break
		} else if path[i] == '"' || path[i] == '\'' {
			i, err = skipString(path, i)
			if err != nil {
				return i, err
			}
			continue
		} else if path[i] == '(' {
			count++
		} else if path[i] == ')' {
			count--
		}
		i++
	}
	if !found {
		return i, errUnexpectedStringEnd
	}
	return i, nil
}

// filterMatch evaluates previously parsed expression and returns boolean to filter out array elements
func filterMatch(input []byte, toks []*xpression.Token) (bool, error) {
	varFunc := func(str []byte, result *xpression.Operand) error {
		if str[0] == '$' {
			// root-based reference has already been evaluated at start
			return nil
		}
		if str[0] != '@' {
			// we only handle item-based references
			result.SetUndefined()
			return nil
		}
		str[0] = '$'
		defer func() { str[0] = '@' }()
		val, err := Get(input, string(str))
		if val == nil || err != nil {
			// not found or other error
			result.SetUndefined()
			return err
		}
		err = decodeValue(val, result)
		if err != nil {
			result.SetUndefined()
		}
		return err
	}

	op, err := xpression.Evaluate(toks, varFunc)
	if err != nil {
		return false, err
	}
	return xpression.ToBoolean(op), nil
}

// decodeValue determine data type of `input` and write parsed value to `op`
func decodeValue(input []byte, op *xpression.Operand) error {
	i, err := skipSpaces(input, 0)
	if err != nil {
		return err
	}
	e, err := skipValue(input, i)
	if err != nil {
		return err
	}
	if bytein(input[i], []byte{'"', '\'', '{', '['}) {
		// string
		op.Type = xpression.StringOperand
		if input[i] == '"' || input[i] == '\'' { // exclude quotes
			i++
			e--
		}
		op.Str = input[i:e]
	} else if (input[i] >= '0' && input[i] <= '9') || input[i] == '-' || input[i] == '.' {
		// number
		f, err := strconv.ParseFloat(string(input[i:e]), 64)
		if err != nil {
			op.Type = xpression.UndefinedOperand
			return err
		}
		op.Type = xpression.NumberOperand
		op.Number = f
	} else {
		// boolean / null (dirty)
		ch := input[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		if ch == 't' || ch == 'f' {
			op.Type = xpression.BooleanOperand
			op.Bool = ch == 't'
		} else {
			op.Type = xpression.NullOperand
		}
	}
	return nil
}
