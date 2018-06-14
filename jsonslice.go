package jsonslice

/**
  JsonSlice 0.3.0
  Michael Gurov, 2018
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

import (
	"errors"
	"strconv"
)

var intSize uintptr

func init() {
}

// Get the jsonpath subset of the input
func Get(input []byte, path string) ([]byte, error) {

	if string(path) == `$` {
		return input, nil
	}

	if path[0] != '$' {
		return nil, errors.New("path: $ expected")
	}

	node, err := parsePath([]byte(path))
	if err != nil {
		return nil, err
	}

	n := node
	for {
		n = n.Next
		if n == nil {
			break
		}
		if n.Filter != nil {
			for _, tok := range n.Filter.toks {
				if tok.Operand != nil && tok.Operand.Node != nil && tok.Operand.Node.Key == "$" {
					val, err := getValue(input, tok.Operand.Node)
					if err != nil {
						// not found or other error
						tok.Operand.Type = cOpBool
						tok.Operand.Bool = false
					}
					decodeValue(val, tok.Operand)
					tok.Operand.Node = nil
				}
			}
		}
	}

	return getValue(input, node)
}

const (
	cArrayType   = 1 << iota // array node
	cArrayRanged = 1 << iota // array properties : ranged [x:y] or indexed [x]
	cIsTerminal  = 1 << iota // terminal node
	cFunction    = 1 << iota // function
	cSubject     = 1 << iota // function subject
	cAgg         = 1 << iota // aggregating
)

type tNode struct {
	Base   byte // $ or @
	Key    string
	Type   int // properties
	Left   int // >=0 index from the start, <0 backward index from the end
	Right  int // 0 till the end inclusive, >0 to index exclusive, <0 backward index from the end exclusive
	Elems  []int
	Next   *tNode
	Filter *tFilter
	Exists bool
}

func bytein(b byte, seq []byte) bool {
	for _, v := range seq {
		if b == v {
			return true
		}
	}
	return false
}

func parsePath(path []byte) (*tNode, error) {
	var err error
	nod := &tNode{}
	i := 0
	l := len(path)
	if l == 0 {
		return nil, errors.New("path: empty")
	}
	// key
	for ; i < l && !bytein(path[i], []byte(" \t.[()]<=>+-*/")); i++ {
	}
	nod.Key = string(path[:i])
	// type
	if i == l {
		nod.Type |= cIsTerminal
		return nod, nil
	}

	if path[i] == '(' && i < l-1 && path[i+1] == ')' {
		// function
		switch nod.Key {
		case "length":
		case "size":
		default:
			return nil, errors.New("path: unknown function " + nod.Key + "()")
		}
		nod.Type |= cFunction
		i += 2
		if i == l {
			nod.Type |= cIsTerminal
			return nod, nil
		}
	} else if path[i] == '[' {
		// array
		nod.Type = cArrayType
		i++ // [
		if i < l-1 && path[i] == '?' && path[i+1] == '(' {
			nod.Type |= cArrayRanged
			i, err = readFilter(path, i+2, nod)
			if err != nil {
				return nil, err
			}
			i++ // )
		} else {
			num := 0
			num, i = readInt(path, i)
			if i == l || (path[i] != ':' && path[i] != ']' && path[i] != ',') {
				return nil, errors.New("path: index bound missing")
			}
			nod.Left = num
			//
			if path[i] == ',' {
				nod.Elems = append(nod.Elems, num)
				for i < l && path[i] != ']' {
					i++
					num, i = readInt(path, i)
					nod.Elems = append(nod.Elems, num)
				}
			} else if path[i] == ':' {
				nod.Type |= cArrayRanged
				i++
				num, ii := readInt(path, i)
				if num == 0 && ii > i {
					return nil, errors.New("path: 0 as a second bound does not make sense")
				}
				if ii == l || path[ii] != ']' {
					return nil, errors.New("path: index bound missing")
				}
				i = ii
				nod.Right = num
			}
		}
		i++ // ]
		if i >= l {
			nod.Type |= cIsTerminal
			return nod, nil
		}
	} else if bytein(path[i], []byte(" \t<=>+-*/)")) {
		nod.Type |= cIsTerminal
		return nod, nil
	}
	if nod.Type&cArrayRanged > 0 && nod.Type&cIsTerminal == 0 {
		nod.Type |= cAgg
	}
	if path[i] != '.' && nod.Type&cIsTerminal == 0 {
		return nil, errors.New("path: invalid element reference ('.' expected)")
	}
	i++
	next, err := parsePath(path[i:])
	if err != nil {
		return nil, err
	}
	nod.Next = next
	if next.Type&cFunction > 0 {
		nod.Type |= cSubject
	}

	return nod, nil
}

func getValue(input []byte, nod *tNode) (result []byte, err error) {

	i, err := skipSpaces(input, 0)
	if err != nil {
		return nil, err
	}

	input = input[i:]
	if len(input) == 0 {
		return nil, errors.New("unexpected end of input")
	}
	if input[0] != '{' && input[0] != '[' {
		return nil, errors.New("object or array expected")
	}
	if nod.Key != "$" && nod.Key != "@" {
		// find the key and seek to the value
		input, err = getKeyValue(input, nod.Key, false)
		if err != nil {
			return nil, err
		}
	}
	// check value type
	if err = checkValueType(input, nod); err != nil {
		return nil, err
	}

	// here we are at the beginning of a value

	if nod.Type&cSubject > 0 {
		return doFunc(input, nod.Next)
	}

	if nod.Type&cIsTerminal > 0 {
		if nod.Type&cArrayType > 0 {
			return sliceArray(input, nod)
		}
		eoe, err := skipValue(input, 0)
		if err != nil {
			return nil, err
		}
		return input[:eoe], nil
	}
	if nod.Type&cArrayType > 0 {
		input, err = sliceArray(input, nod)
		if err != nil {
			return nil, err
		}
		if nod.Type&cAgg > 0 {
			return getNodes(input, nod.Next)
		}
	}
	return getValue(input, nod.Next)
}

func getNodes(input []byte, nod *tNode) ([]byte, error) {
	var err error
	var value []byte
	var e int
	l := len(input)
	i := 1 // skip '['

	i, err = skipSpaces(input, i)
	if err != nil {
		return nil, err
	}
	// scan for elements
	var result []byte
	for i < l && input[i] != ']' {
		value, err = getValue(input[i:], nod)
		if err == nil {
			if len(result) == 0 {
				result = []byte{'['}
			} else {
				result = append(result, ',')
			}
			result = append(result, value...)
		}
		// skip value
		e, err = skipValue(input, i)
		if err != nil {
			return nil, err
		}
		// skip spaces after value
		i, err = skipSpaces(input, e)
		if err != nil {
			return nil, err
		}
	}
	if len(result) > 0 {
		result = append(result[:len(result):len(result)], ']')
	}
	return result, nil
}

const keySeek = 1
const keyOpen = 2
const keyClose = 4

// getKeyValue: find the key and seek to the value. Cut value if needed
func getKeyValue(input []byte, key string, cut bool) ([]byte, error) {
	var err error
	var ch byte
	if input[0] != '{' {
		return nil, errors.New("object expected")
	}

	i := 1
	e := 0
	l := len(input)
	k := make([]byte, 0, 32)

	for i < l && input[i] != '}' {
		state := keySeek
		k = k[:0]
		for i < l && state != keyClose {
			ch = input[i]
			if ch == '"' {
				if state == keySeek {
					state = keyOpen
				} else if state == keyOpen {
					state = keyClose
				}
			} else if state == keyOpen {
				k = append(k, byte(ch))
			}
			i++
		}

		if state == keyClose {
			i, err = seekToValue(input, i)
			if err != nil {
				return nil, err
			}
			if key == string(k) {
				if cut {
					e, err = skipValue(input, i)
					if err != nil {
						return nil, err
					}
					return input[i:e], nil
				}
				return input[i:], nil
			}
			e, err = skipValue(input, i)
			if err != nil {
				return nil, err
			}
			i, err = skipSpaces(input, e)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, errors.New(`"` + key + `" field not found`)
}

type tElem struct {
	start int
	end   int
}

// sliceArray select node(s) by bound(s)
func sliceArray(input []byte, nod *tNode) ([]byte, error) {
	l := len(input)
	if input[0] != '[' {
		return nil, errors.New("array not found at " + nod.Key)
	}
	i := 1 // skip '['
	// select by positive index -- easiest case
	if nod.Type&cArrayRanged == 0 && nod.Left >= 0 && len(nod.Elems) == 0 {
		ielem := 0
		for i < l && input[i] != ']' {
			e, err := skipValue(input, i)
			if err != nil {
				return nil, err
			}
			if ielem == nod.Left {
				return input[i:e], nil
			}
			// skip spaces after value
			i, err = skipSpaces(input, e)
			if err != nil {
				return nil, err
			}
			ielem++
		}
		return nil, errors.New("specified element not found")
	}
	elems := make([]tElem, 0, 32)
	result := []byte{'['}
	// fullscan
	for i < l && input[i] != ']' {
		e, err := skipValue(input, i)
		if err != nil {
			return nil, err
		}
		if nod.Filter != nil {
			b, err := filterMatch(input[i:e], nod.Filter.toks)
			if err != nil {
				return nil, err
			}
			if b {
				if len(result) > 2 {
					result = append(result, ',')
				}
				result = append(result, input[i:e]...)
			}
		}
		elems = append(elems, tElem{i, e})
		// skip spaces after value
		i, err = skipSpaces(input, e)
		if err != nil {
			return nil, err
		}
	}
	if nod.Filter != nil {
		return append(result, ']'), nil
	}
	if len(nod.Elems) > 0 {
		result := []byte{'['}
		for _, ii := range nod.Elems {
			if len(result) > 2 {
				result = append(result, ',')
			}
			result = append(result, input[elems[ii].start:elems[ii].end]...)
		}
		return append(result, ']'), nil
	}
	//   select by index(es)
	if nod.Type&cArrayRanged == 0 {
		a := nod.Left
		a += len(elems) // nod.Left is negative, so corrent it to a real element index
		if a < 0 || a >= len(elems) {
			return nil, errors.New(nod.Key + "[" + strconv.Itoa(nod.Left) + "] does not exist")
		}
		return input[elems[a].start:elems[a].end], nil
	}
	// two bounds
	a := nod.Left
	b := nod.Right
	if b == 0 {
		b = len(elems)
	}
	if a < 0 {
		a += len(elems)
	}
	if b < 0 {
		b += len(elems)
	}
	b-- // right bound excluded
	if a < 0 || a >= len(elems) || b < 0 || b >= len(elems) {
		return nil, errors.New(nod.Key + "[" + strconv.Itoa(nod.Left) + ":" + strconv.Itoa(nod.Right) + "] does not exist")
	}
	input = input[elems[a].start:elems[b].end]
	input = input[:len(input):len(input)]
	return append([]byte{'['}, append(input, ']')...), nil
}

// sliceValue: slice a single value
func sliceValue(input []byte) ([]byte, error) {
	eoe, err := skipValue(input, 0)
	if err != nil {
		return nil, err
	}
	return input[:eoe], nil
}

// getValues: get (sub-)values from array
func getValues(input []byte, nod *tNode) ([]byte, error) {
	return nil, errors.New("not yet supported")
}

func seekToValue(input []byte, i int) (int, error) {
	var err error
	// spaces before ':'
	i, err = skipSpaces(input, i)
	if err != nil {
		return 0, err
	}
	if input[i] != ':' {
		return 0, errors.New("\":\" expected")
	}
	i++ // colon
	return skipSpaces(input, i)
}

func skipValue(input []byte, i int) (int, error) {
	var err error
	// spaces
	i, err = skipSpaces(input, i)
	if err != nil {
		return 0, err
	}

	l := len(input)
	if i >= l {
		return i, nil
	}
	if input[i] == '"' {
		// string
		return skipString(input, i)
	} else if input[i] == '{' || input[i] == '[' {
		// object or array
		mark := input[i]
		unmark := mark + 2
		nested := 0
		instr := false
		prev := mark
		i++
		for i < l && !(input[i] == unmark && nested == 0) {
			ch := input[i]
			if ch == '"' {
				if prev != '\\' {
					instr = !instr
				}
			} else if !instr {
				if ch == mark {
					nested++
				} else if ch == unmark {
					nested--
				}
			}
			prev = ch
			i++
		}
		if i == l {
			return 0, errors.New("unexpected end of input")
		}
		i++ // closing mark
	} else {
		if (input[i] >= '0' && input[i] <= '9') || input[i] == '-' || input[i] == '.' {
			// number
			for ; i < l; i++ {
				ch := input[i]
				if !((ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == 'E' || ch == 'e') {
					break
				}
			}
		} else {
			// bool, null
			vals := [][]byte{[]byte("true"), []byte("false"), []byte("null")}
			v := -1
			p := 0
			for ; i < l; i++ {
				ch := input[i]
				if ch >= 'A' && ch <= 'Z' {
					ch += 'a' - 'A'
				}
				if v == -1 {
					for iv, vv := range vals {
						if ch == vv[0] {
							v = iv
						}
					}
					if v == -1 {
						return i, errors.New("unexpected value")
					}
				} else if vals[v][p] != ch {
					return i, errors.New("unexpected value")
				}
				p++
				if p == len(vals[v]) {
					break
				}
			}
		}
	}
	return i, nil
}

func checkValueType(input []byte, nod *tNode) error {
	if len(input) < 2 {
		return errors.New("unexpected end of input")
	}
	if nod.Type&cSubject > 0 {
		return nil
	}
	ch := input[0]
	if ch == '[' && nod.Type&cArrayType == 0 && nod.Type&cIsTerminal == 0 {
		return errors.New("object expected at " + nod.Key)
	} else if ch == '{' && nod.Type&cArrayType > 0 {
		return errors.New("array expected at " + nod.Key)
	} else if ch != '{' && ch != '[' && nod.Type&cIsTerminal == 0 {
		return errors.New("complex type expected at " + nod.Key)
	}
	return nil
}

// get input and current value position
// return next key and new value position

func nextKey(input []byte, i int) ([]byte, int) {
	status := keySeek
	key := make([]byte, 0, 32)
	for l := len(input); i < l; i++ {
		ch := input[i]
		switch {
		case status&keyOpen > 0:
			if ch == '"' {
				status -= keyOpen
				status |= keyClose
			} else {
				key = append(key, ch)
			}
		case status&keySeek > 0 && ch == '"':
			status -= keySeek
			status |= keyOpen
		case status&keyClose > 0 && ch == ':':
			return key, i + 1
		}
	}
	return nil, i
}

func doFunc(input []byte, nod *tNode) ([]byte, error) {
	var err error
	var result int
	switch nod.Key {
	case "size":
		result, err = skipValue(input, 0)
	case "length":
		if input[0] == '"' {
			result, err = skipString(input, 0)
		} else if input[0] == '[' {
			i := 1
			l := len(input)
			// count elements
			for i < l && input[i] != ']' {
				e, err := skipValue(input, i)
				if err != nil {
					return nil, err
				}
				result++
				// skip spaces after value
				i, err = skipSpaces(input, e)
				if err != nil {
					return nil, err
				}
			}
		} else {
			return nil, errors.New("length() is only applicable to array or string")
		}
	}
	if err != nil {
		return nil, err
	}
	return []byte(strconv.Itoa(result)), nil
}

func readInt(path []byte, i int) (int, int) {
	sign := 1
	l := len(path)
	if i >= l {
		return 0, i
	}
	ind := 0
	for i < l && (path[i] == '-' || (path[i] >= '0' && path[i] <= '9')) {
		ch := path[i]
		if ch == '-' {
			sign = -1
		} else {
			ind = ind*10 + int(ch-'0')
		}
		i++
	}
	return ind * sign, i
}

func skipSpaces(input []byte, i int) (int, error) {
	l := len(input)
	for ; i < l; i++ {
		ch := input[i]
		if !(ch == ',' || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n') {
			break
		}
	}
	if i == l {
		return 0, errors.New("unexpected end of input")
	}
	return i, nil
}

func skipString(input []byte, i int) (int, error) {
	prev := byte('"')
	done := false
	i++
	l := len(input)
	for i < l && !done {
		ch := input[i]
		if ch == '"' && prev != '\\' {
			done = true
		}
		prev = ch
		i++
	}
	if i == l && !done {
		return 0, errors.New("unexpected end of input")
	}
	return i, nil
}
