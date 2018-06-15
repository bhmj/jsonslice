package jsonslice

import (
	"errors"
	"strconv"
)

const (
	cOpNone   = 1 << iota
	cOpNumber = 1 << iota
	cOpString = 1 << iota
	cOpBool   = 1 << iota
	cOpNull   = 1 << iota
)

/*
var operatorFn map[byte]

func init() {
	setupOperatorMap()
}
*/

/*
  <operand> [ <operator> <operand> ] [ <compare> <operand> [ <operator> <operand> ] ]

  <filter> : <expression> [ <operator> <expression> ]   <--- .single
  <expression> : <operand> [ <operator> <operand> ]
  <compare> : /(==)|(!=)|(>)|(<)|(>=)|(<=)/
  <operand> : <number> | <string> | <bool> | <jsonpath>
  <number> : /-?[0-9]+(\.[0-9]*?)?((E|e)[0-9]+)?/
  <string> : /"[^"]*"/
  <bool> : /(true)|(false)/
  <jsonpath> : /[@$].+/           <--- .exists
  <operator> : /[+-/*] | (>=,<=,==,!=,>,<) | (&&,||)/
*/

type tFilter struct {
	toks []*tToken
}
type tToken struct {
	Operand  *tOperand
	Operator byte
}
type tOperand struct {
	Type   int // cOp*
	Number float64
	Bool   bool
	Str    []byte
	Node   *tNode
}

var operator = [...]string{">=", "<=", "==", "!=", ">", "<", "&&", "||"}
var operatorCode = [...]byte{'G', 'L', 'E', 'N', 'g', 'l', '&', '|'}
var operatorPrecedence = map[byte]int{'&': 1, '|': 1, 'g': 2, 'l': 2, 'E': 2, 'N': 2, 'G': 2, 'L': 2, '+': 3, '-': 3, '*': 4, '/': 4}

type stack struct {
	s []*tToken
}

func reverse(s []*tToken) []*tToken {
	l := len(s)
	for i := 0; i < l/2; i++ {
		s[i], s[l-i-1] = s[l-i-1], s[i]
	}
	return s
}

func (s *stack) push(tok *tToken) {
	s.s = append(s.s, tok)
}
func (s *stack) pop() *tToken {
	l := len(s.s)
	if l == 0 {
		return nil
	}
	v := (s.s)[l-1]
	s.s = (s.s)[:l-1]
	return v
}
func (s *stack) peek() (tok *tToken) {
	l := len(s.s)
	if l == 0 {
		return nil
	}
	return (s.s)[l-1]
}
func (s *stack) get() []*tToken {
	return s.s
}

func readFilter(path []byte, i int, nod *tNode) (int, error) {
	l := len(path)

	// lexer
	tokens := make([]*tToken, 0)
	var tok *tToken
	var err error
	prevOperator := true
	for i < l && path[i] != ')' {
		i, tok, err = nextToken(path, i, prevOperator)
		if err != nil {
			return i, err
		}
		if tok != nil {
			prevOperator = tok.Operator != 0
			tokens = append(tokens, tok)
		}
	}

	//parser
	opStack := new(stack)
	result := new(stack)
	for t := len(tokens) - 1; t >= 0; t-- {
		op := tokens[t]
		if op.Operand != nil {
			result.push(op)
		} else {
			for {
				top := opStack.peek()
				if top != nil && operatorPrecedence[top.Operator] >= operatorPrecedence[op.Operator] {
					result.push(opStack.pop())
					continue
				}
				break
			}
			opStack.push(op)
		}
	}
	for {
		top := opStack.pop()
		if top == nil {
			break
		}
		result.push(top)
	}

	nod.Filter = &tFilter{toks: reverse(result.get())}

	return i, nil
}

// operand = (number, string, node), operator, compare
func nextToken(path []byte, i int, prevOperator bool) (int, *tToken, error) {
	var err error
	var tok *tToken
	l := len(path)
	for i < l && path[i] != ')' {
		i, err = skipSpaces(path, i)
		if err != nil {
			return 0, nil, err
		}
		if path[i] == ')' {
			return i, tok, nil
		}
		// number
		if (path[i] >= '0' && path[i] <= '9') || (path[i] == '-' && prevOperator) {
			return readNumber(path, i)
		}
		// string
		if path[i] == '"' {
			return readString(path, i)
		}
		// bool
		if path[i] == 't' || path[i] == 'f' {
			return readString(path, i)
		}
		// jsonpath node
		if path[i] == '@' || path[i] == '$' {
			nod, err := parsePath(path[i:])
			if err != nil {
				return 0, nil, err
			}
			for i < l && !bytein(path[i], []byte(" \t<=>+-*/)&|")) {
				i++
			}
			return i, &tToken{Operand: &tOperand{Type: cOpNone, Node: nod}}, nil
		}
		// operator
		if path[i] == '+' || path[i] == '-' || path[i] == '*' || path[i] == '/' {
			return i + 1, &tToken{Operator: path[i]}, nil
		}
		// compare
		if i >= l-1 {
			return i, nil, errors.New("unexpected end of token")
		}
		for ic, cmp := range operator {
			if string(path[i:i+len(cmp)]) == cmp {
				return i + len(cmp), &tToken{Operator: operatorCode[ic]}, nil
			}
		}
		return 0, nil, errors.New("unknown token " + string(path[i]))
	}

	return i, tok, nil
}

func readNumber(path []byte, i int) (int, *tToken, error) {
	e, err := skipValue(path, i)
	if err != nil {
		return e, nil, err
	}
	s, err := strconv.ParseFloat(string(path[i:e]), 64)
	if err != nil {
		return e, nil, err
	}
	return e, &tToken{Operand: &tOperand{Type: cOpNumber, Number: s}}, nil
}

func readString(path []byte, i int) (int, *tToken, error) {
	i++ // quote
	s := i
	l := len(path)
	for i < l && path[i] != '"' {
		if path[i] == '\\' {
			i += 2
			continue
		}
		i++
	}
	if i == l {
		return i, nil, errors.New("unexpected end of string")
	}
	e := i
	i++ // unquote

	return i, &tToken{Operand: &tOperand{Type: cOpString, Str: path[s:e]}}, nil
}

func readBool(path []byte, i int) (int, *tToken, error) {
	s := i
	l := len(path)
	t, f := []byte("true\x00\x00"), []byte("false\x00")
	for i < l && (path[i] == t[i-s] || path[i] == f[i-s]) && (t[i-s] > 0 || f[i-s] > 0) {
		i++
	}
	if i == l || t[i-s] > 0 && f[i-s] > 0 {
		return i, nil, errors.New("invalid boolean value")
	}

	return i, &tToken{Operand: &tOperand{Type: cOpBool, Bool: path[s] == 't'}}, nil
}

// filterMatch
func filterMatch(input []byte, toks []*tToken) (bool, error) {
	if len(toks) == 0 {
		return false, errors.New("invalid filter")
	}
	op, _, err := evalToken(input, toks)
	if err != nil {
		return false, err
	}
	switch op.Type {
	case cOpNull:
		return false, nil
	case cOpBool:
		return op.Bool, nil
	case cOpNumber:
		return op.Number > 0, nil
	case cOpString:
		return len(op.Str) > 0, nil
	default:
		return false, nil
	}
}

func evalToken(input []byte, toks []*tToken) (*tOperand, []*tToken, error) {
	if len(toks) == 0 {
		return nil, toks, errors.New("not enough arguments")
	}
	tok := toks[0]
	if tok.Operand != nil {
		if tok.Operand.Node != nil {
			val, err := getValue(input, tok.Operand.Node)
			if err != nil {
				// not found or other error
				tok.Operand.Type = cOpNull
				return tok.Operand, toks[1:], nil
			}
			return tok.Operand, toks[1:], decodeValue(val, tok.Operand)
		}
		return tok.Operand, toks[1:], nil
	}
	var (
		err   error
		left  *tOperand
		right *tOperand
	)
	left, toks, err = evalToken(input, toks[1:])
	if err != nil {
		return nil, toks, err
	}
	right, toks, err = evalToken(input, toks)
	if err != nil {
		return nil, toks, err
	}

	op, err := execOperator(tok.Operator, left, right)
	return op, toks, err
}

func decodeValue(input []byte, op *tOperand) error {
	i, err := skipSpaces(input, 0)
	if err != nil {
		return err
	}
	e, err := skipValue(input, i)
	if err != nil {
		return err
	}
	if input[i] == '"' || input[i] == '{' || input[i] == '[' {
		// string
		op.Type = cOpString
		if input[i] == '"' { // exclude quotes
			i++
			e--
		}
		op.Str = input[i:e]
	} else if (input[i] >= '0' && input[i] <= '9') || input[i] == '-' || input[i] == '.' {
		// number
		f, err := strconv.ParseFloat(string(input[i:e]), 64)
		if err != nil {
			op.Type = cOpNone
			return err
		}
		op.Type = cOpNumber
		op.Number = f
	} else {
		//
		ch := input[i]
		if ch >= 'A' && ch <= 'Z' {
			ch += 'a' - 'A'
		}
		if ch == 't' || ch == 'f' {
			op.Type = cOpBool
			op.Bool = ch == 't'
		} else {
			op.Type = cOpNull
		}
	}
	return nil
}

func execOperator(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand

	if op == '+' || op == '-' || op == '*' || op == '/' {
		// arithmetic
		if left.Type != cOpNumber || right.Type != cOpNumber {
			return nil, errors.New("invalid operands for " + string(op))
		}
		res.Type = left.Type
		switch op {
		case '+':
			res.Number = left.Number + right.Number
		case '-':
			res.Number = left.Number - right.Number
		case '*':
			res.Number = left.Number * right.Number
		case '/':
			res.Number = left.Number / right.Number
		}
		return &res, nil
	} else if op == 'g' || op == 'l' || op == 'E' || op == 'N' || op == 'G' || op == 'L' {
		// comparision
		res.Type = cOpBool
		if left.Type == cOpNull || right.Type == cOpNull {
			res.Bool = false
			return &res, nil
		}
		if left.Type != right.Type {
			return nil, errors.New("operand types do not match")
		}
		switch left.Type {
		case cOpBool:
			switch op {
			case 'g':
				res.Bool = (left.Bool && !right.Bool)
			case 'l':
				res.Bool = (!left.Bool && right.Bool)
			case 'E':
				res.Bool = (left.Bool == right.Bool)
			case 'N':
				res.Bool = (left.Bool != right.Bool)
			case 'G':
			case 'L':
				res.Bool = right.Bool
			}
		case cOpNumber:
			switch op {
			case 'g':
				res.Bool = left.Number > right.Number
			case 'l':
				res.Bool = left.Number < right.Number
			case 'E':
				res.Bool = left.Number == right.Number
			case 'N':
				res.Bool = left.Number != right.Number
			case 'G':
				res.Bool = left.Number >= right.Number
			case 'L':
				res.Bool = left.Number <= right.Number
			}
		case cOpString:
			switch op {
			case 'E':
				res.Bool = compareSlices(left.Str, right.Str) == 0
			case 'N':
				res.Bool = compareSlices(left.Str, right.Str) != 0
			default:
				return left, errors.New("operator is not applicable to strings")
			}
		}
		return &res, nil
	} else if op == '&' || op == '|' {
		// logic
		res.Type = cOpBool
		if left.Type == cOpNull || right.Type == cOpNull {
			res.Bool = false
			return &res, nil
		}
		l := false
		r := false
		switch left.Type {
		case cOpBool:
			l = left.Bool
		case cOpNumber:
			l = left.Number != 0
		case cOpString:
			l = len(left.Str) > 0
		}
		switch right.Type {
		case cOpBool:
			r = right.Bool
		case cOpNumber:
			r = right.Number != 0
		case cOpString:
			r = len(right.Str) > 0
		}
		if op == '&' {
			res.Bool = l && r
		} else {
			res.Bool = l || r
		}
		return &res, nil
	}
	return &res, errors.New("unknown operator")
}

func compareSlices(s1 []byte, s2 []byte) int {
	if len(s1) != len(s2) {
		return len(s1) - len(s2)
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return int(s1[i] - s2[i])
		}
	}
	return 0
}