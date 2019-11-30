package jsonslice

import (
	"regexp"
	"strconv"
)

const (
	cOpNone   = 1 << iota
	cOpNumber = 1 << iota
	cOpString = 1 << iota
	cOpBool   = 1 << iota
	cOpNull   = 1 << iota
	cOpRegexp = 1 << iota
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
	Regexp *regexp.Regexp
}

var operator = [...]string{">=", "<=", "==", "!=~", "!~", "!=", "=~", ">", "<", "&&", "||"}
var operatorCode = [...]byte{'G', 'L', 'E', 'r', 'r', 'N', 'R', 'g', 'l', '&', '|'}
var operatorPrecedence = map[byte]int{'&': 1, '|': 1, 'g': 2, 'l': 2, 'E': 2, 'N': 2, 'R': 2, 'r': 2, 'G': 2, 'L': 2, '+': 3, '-': 3, '*': 4, '/': 4}

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
	prevOperator := byte('+')
	for i < l && path[i] != ')' {
		i, tok, err = nextToken(path, i, prevOperator)
		if err != nil {
			return i, err
		}
		if tok != nil {
			prevOperator = tok.Operator
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
	nod.Type |= cFilter
	nod.Type &^= cDot

	if i < l {
		i++ // ')'
		if i < l && path[i] == ']' {
			i++
		}
	}
	return i, nil
}

// operand = (number, string, node), operator, compare
func nextToken(path []byte, i int, prevOperator byte) (int, *tToken, error) {
	var err error
	var tok *tToken
	l := len(path)
	for i < l && path[i] != ')' {
		i, err = skipSpaces(path, i)
		if err != nil {
			return 0, nil, err
		}
		// end of filter
		if path[i] == ')' {
			break
		}
		// regexp
		if path[i] == '/' && prevOperator&^('r'-'R') == 'R' {
			return readRegexp(path, i)
		}
		// number
		if (path[i] >= '0' && path[i] <= '9') || (path[i] == '-' && prevOperator != 0) {
			return readNumber(path, i)
		}
		// string
		if path[i] == '"' || path[i] == '\'' {
			return readString(path, i)
		}
		// bool
		if path[i] == 't' || path[i] == 'f' {
			return readBool(path, i)
		}
		return tokComplex(path, i)
	}
	return i, tok, nil
}

func tokComplex(path []byte, i int) (int, *tToken, error) {
	l := len(path)
	// jsonpath node
	if path[i] == '@' || path[i] == '$' {
		nod, j, err := readRef(path[i:], 1, 0)
		if err != nil {
			return 0, nil, err
		}
		if path[i] == '$' {
			nod.Type |= cRoot
		}
		i += j
		return i, &tToken{Operand: &tOperand{Type: cOpNone, Node: nod}}, nil
	}
	// operator
	if bytein(path[i], []byte{'+', '-', '*', '/'}) {
		return i + 1, &tToken{Operator: path[i]}, nil
	}
	// compare
	if i >= l-1 {
		return i, nil, errUnexpectedEOT
	}
	for ic, cmp := range operator {
		if string(path[i:i+len(cmp)]) == cmp {
			return i + len(cmp), &tToken{Operator: operatorCode[ic]}, nil
		}
	}
	return i, nil, errUnknownToken
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
	bound := path[i]
	prev := bound
	i++ // quote
	s := i
	l := len(path)
	for i < l {
		ch := path[i]
		if ch == bound {
			if prev != '\\' {
				break
			}
		}
		prev = ch
		i++
	}
	if i == l {
		return i, nil, errUnexpectedStringEnd
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
		return i, nil, errInvalidBoolean
	}

	return i, &tToken{Operand: &tOperand{Type: cOpBool, Bool: path[s] == 't'}}, nil
}

func readRegexp(path []byte, i int) (int, *tToken, error) {
	l := len(path)
	prev := byte(0)
	re := make([]byte, 0, 32)
	flags := make([]byte, 0, 8)
	i++
	for i < l && !(path[i] == '/' && prev != '\\') {
		prev = path[i]
		re = append(re, prev)
		i++
	}
	if i < l { // skip trailing '/'
		i++
	}
	flags = append(flags, '(', '?')
	for i < l && len(flags) < 4 && (path[i] == 'i' || path[i] == 'm' || path[i] == 's' || path[i] == 'U') {
		flags = append(flags, path[i])
		i++
	}
	flags = append(flags, ')')
	rex := ""
	if len(flags) > 3 {
		rex = string(flags) + string(re)
	} else {
		rex = string(re)
	}
	reg, err := regexp.Compile(rex)
	if err != nil {
		return i, nil, err
	}
	return i, &tToken{Operand: &tOperand{Type: cOpRegexp, Regexp: reg}}, nil
}

// filterMatch
func filterMatch(input []byte, toks []*tToken) (bool, error) {
	if len(toks) == 0 {
		return false, errEmptyFilter
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
		return nil, toks, errNotEnoughArguments
	}
	tok := toks[0]
	if tok.Operand != nil {
		if tok.Operand.Node != nil {
			val, err := getValue(input, tok.Operand.Node, false)
			if len(val) == 0 || err != nil {
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
	if bytein(input[i], []byte{'"', '\'', '{', '['}) {
		// string
		op.Type = cOpString
		if input[i] == '"' || input[i] == '\'' { // exclude quotes
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
		return opArithmetic(op, left, right)
	} else if op == 'g' || op == 'l' || op == 'E' || op == 'N' || op == 'G' || op == 'L' || op == 'R' || op == 'r' {
		// comparison
		return opComparison(op, left, right)
	} else if op == '&' || op == '|' {
		// logic
		return opLogic(op, left, right)
	}
	return &res, errUnknownOperator
}

func opArithmetic(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand

	if left.Type != cOpNumber || right.Type != cOpNumber {
		return nil, errInvalidArithmetic
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
}

func opComparison(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand

	res.Type = cOpBool
	if left.Type == cOpNull || right.Type == cOpNull {
		res.Bool = false
		return &res, nil
	}
	if op == 'R' || op == 'r' {
		if !(left.Type == cOpString && right.Type == cOpRegexp) {
			return nil, errInvalidRegexp
		}
	} else if left.Type != right.Type {
		return nil, errOperandTypes
	}
	switch left.Type {
	case cOpBool:
		return opComparisonBool(op, left, right)
	case cOpNumber:
		return opComparisonNumber(op, left, right)
	case cOpString:
		return opComparisonString(op, left, right)
	}
	return &res, nil
}

func opComparisonBool(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand

	res.Type = cOpBool
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
	return &res, nil
}

func opComparisonNumber(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand
	res.Type = cOpBool
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
	return &res, nil
}

func opComparisonString(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand
	res.Type = cOpBool
	switch op {
	case 'E':
		res.Bool = compareSlices(left.Str, right.Str) == 0
	case 'N':
		res.Bool = compareSlices(left.Str, right.Str) != 0
	case 'R':
		res.Bool = right.Regexp.MatchString(string(left.Str))
	case 'r':
		res.Bool = !right.Regexp.MatchString(string(left.Str))
	default:
		return left, errInvalidOperatorStrings
	}
	return &res, nil
}

func opLogic(op byte, left *tOperand, right *tOperand) (*tOperand, error) {
	var res tOperand
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
