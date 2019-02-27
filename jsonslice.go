package jsonslice

/**
  JsonSlice 0.7.3
  Michael Gurov, 2018-2019
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

import (
	"bytes"
	"errors"
	"strconv"
	"sync"
)

var (
	nodePool sync.Pool
)

func init() {
	nodePool = sync.Pool{
		New: func() interface{} {
			return &tNode{
				Keys:  make([]word, 0),
				Elems: make([]int, 0),
			}
		},
	}
}

func getEmptyNode() *tNode {
	nod := nodePool.Get().(*tNode)
	nod.Elems = nod.Elems[:0]
	nod.Exists = false
	nod.Filter = nil
	nod.Key = nod.Key[:0]
	nod.Keys = nod.Keys[:0]
	nod.Left = 0
	nod.Next = nil
	nod.Right = 0
	nod.Type = 0
	return nod
}

// Get returns a part of input, matching jsonpath.
// In terms of allocations there are two cases of retreiving data from the input:
// 1. (simple case) the result is a simple subslice of a source input.
// 2. the result is a merge of several non-contiguous parts of input. More allocations are needed.
func Get(input []byte, path string) ([]byte, error) {

	if len(path) == 0 {
		return nil, errors.New("path: empty")
	}

	if len(path) == 1 && path[0] == '$' {
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
				if tok.Operand != nil && tok.Operand.Node != nil && len(tok.Operand.Node.Key) == 1 && tok.Operand.Node.Key[0] == '$' {
					val, err := getValue(input, tok.Operand.Node)
					if err != nil {
						// not found or other error
						tok.Operand.Type = cOpNull
					}
					decodeValue(val, tok.Operand)
					tok.Operand.Node = nil
				}
			}
		}
	}

	result, err := getValue(input, node)

	// return nodes back to pool
	for {
		if node == nil {
			break
		}
		p := node.Next
		nodePool.Put(node)
		node = p
	}

	return result, err
}

const (
	cArrayType   = 1 << iota // array node
	cArrayRanged = 1 << iota // array properties : ranged [x:y] or indexed [x]
	cIsTerminal  = 1 << iota // terminal node
	cFunction    = 1 << iota // function
	cSubject     = 1 << iota // function subject
	cAgg         = 1 << iota // aggregating
	cDeep        = 1 << iota // deepscan
)

type word []byte

type tNode struct {
	Key    word
	Keys   []word
	Type   int // properties
	Left   int // >=0 index from the start, <0 backward index from the end
	Right  int // 0 till the end inclusive, >0 to index exclusive, <0 backward index from the end exclusive
	Elems  []int
	Next   *tNode
	Filter *tFilter
	Exists bool
}

// returns true if b matches one of the elements of seq
func bytein(b byte, seq []byte) bool {
	for i := 0; i < len(seq); i++ {
		if b == seq[i] {
			return true
		}
	}
	return false
}

var keyTerminator = []byte{' ', '\t', '.', '[', '(', ')', ']', '<', '=', '>', '+', '-', '*', '/', '&', '|'}

// parse jsonpath and return a root of a linked list of nodes
func parsePath(path []byte) (*tNode, error) {
	var err error
	var done bool
	var nod *tNode

	i := 0
	l := len(path)

	if l == 0 {
		return nil, errors.New("path: unexpected end of path")
	}

	//	if path[0] != '$' && path[0] != 's' {
	//		return getEmptyNode(), nil
	//	}
	// get key
	if path[i] == '*' {
		i++
	} else {
		for ; i < l && !bytein(path[i], keyTerminator); i++ {
		}
	}

	nod = getEmptyNode()
	nod.Key = path[:i]

	//	if path[0] == 's' {
	//		return getEmptyNode(), nil
	//	}
	if i == l {
		// finished parsing
		nod.Type |= cIsTerminal
		return nod, nil
	}

	// get node type and also get Keys if specified
	done, i, err = nodeType(path, i, nod)
	if len(nod.Key) == 0 && len(nod.Keys) == 1 {
		nod.Key = nod.Keys[0]
		nod.Keys = nil
	}
	if len(nod.Key) != 0 && len(nod.Keys) > 0 {
		mid := nod
		nod := getEmptyNode()
		nod.Keys = mid.Keys
		nod.Type = mid.Type
		mid.Type = mid.Type & (^cIsTerminal)
		mid.Next = nod
	}
	if done || err != nil {
		return nod, err
	}

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

var pathTerminator = []byte{' ', '\t', '<', '=', '>', '+', '-', '*', '/', ')', '&', '|'}

func nodeType(path []byte, i int, nod *tNode) (bool, int, error) {
	var err error
	l := len(path)
	if path[i] == '(' && i < l-1 && path[i+1] == ')' {
		// function
		return detectFn(path, i, nod)
	} else if path[i] == '[' {
		// array
		i, err = parseArrayIndex(path, i, nod)
		if err != nil {
			return true, i, err
		}
		if nod.Type&cIsTerminal > 0 {
			return true, i, nil
		}
	}
	ch := path[i]
	if bytein(ch, pathTerminator) {
		nod.Type |= cIsTerminal
		return true, i, nil
	}
	if ch == '.' {
		i++
		if i == l {
			return true, i, errors.New("path: unexpected end of path")
		}
		if path[i] == '.' {
			nod.Type |= cDeep
			i++
		}
	} else if ch != '[' { // nested array
		return true, i, errors.New("path: invalid element reference")
	}
	return false, i, nil
}

func detectFn(path []byte, i int, nod *tNode) (bool, int, error) {
	if !(bytes.EqualFold(nod.Key, []byte("length")) ||
		bytes.EqualFold(nod.Key, []byte("count")) ||
		bytes.EqualFold(nod.Key, []byte("size"))) {
		return true, i, errors.New("path: unknown function")
	}
	nod.Type |= cFunction
	i += 2
	if i == len(path) {
		nod.Type |= cIsTerminal
	}
	return true, i, nil
}

func parseArrayIndex(path []byte, i int, nod *tNode) (int, error) {
	l := len(path)
	var err error
	i++ // [
	if i < l && path[i] == '\'' {
		return parseKeyList(path, i, nod)
	}
	nod.Type = cArrayType
	if i < l-1 && path[i] == '?' && path[i+1] == '(' {
		// filter
		nod.Type |= cArrayRanged | cAgg
		i, err = readFilter(path, i+2, nod)
		if err != nil {
			return i, err
		}
		i++ // )
	} else {
		// single index, slice or index list
		i, err = readArrayIndex(path, i, nod)
		if err != nil {
			return i, err
		}
	}
	if i >= l || path[i] != ']' {
		return i, errors.New("path: index bound missing")
	}
	i++ // ]
	if i == l {
		nod.Type |= cIsTerminal
	}
	return i, nil
}

func parseKeyList(path []byte, i int, nod *tNode) (int, error) {
	l := len(path)
	// now at '
	for i < l && path[i] != ']' {
		i++ // skip '
		e := i
		for ; e < l && path[e] != '\''; e++ {
		}
		if e == l {
			return i, errors.New("path: key list terminated unexpectedly")
		}
		nod.Keys = append(nod.Keys, path[i:e])
		i = e + 1 // skip '
		for ; i < l && path[i] != '\'' && path[i] != ']'; i++ {
		} // sek to next ' or ]
	}
	if i == l {
		return i, errors.New("path: key list terminated unexpectedly")
	}
	i++ // ]
	if i == l {
		nod.Type |= cIsTerminal
	}
	return i, nil
}

func readArrayIndex(path []byte, i int, nod *tNode) (int, error) {
	l := len(path)
	num := 0
	num, i = readInt(path, i)
	if i == l || !bytein(path[i], []byte{':', ',', ']'}) {
		return i, errors.New("path: index bound missing")
	}
	nod.Left = num

	switch path[i] {
	case ',':
		nod.Type |= cArrayRanged | cAgg
		nod.Elems = append(nod.Elems, num)
		for i < l && path[i] != ']' {
			i++
			num, i = readInt(path, i)
			nod.Elems = append(nod.Elems, num)
		}
	case ':':
		nod.Type |= cArrayRanged | cAgg
		i++
		num, ii := readInt(path, i)
		if ii-i > 0 && num == 0 {
			return i, errors.New("path: 0 as a second bound does not make sense")
		}
		i = ii
		nod.Right = num
	}
	return i, nil
}

func getValue(input []byte, nod *tNode) (result []byte, err error) {

	i, _ := skipSpaces(input, 0)

	input = input[i:]
	if len(input) == 0 {
		return nil, errors.New("unexpected end of input")
	}
	if !bytein(input[0], []byte{'{', '['}) {
		return nil, errors.New("object or array expected")
	}
	// wildcard
	if nod.Key != nil && len(nod.Key) == 1 && nod.Key[0] == '*' {
		return wildScan(input, nod)
	}
	if len(nod.Keys) > 0 || (len(nod.Key) > 0 && nod.Key[0] != '$' && nod.Key[0] != '@') {
		// find the key and seek to the value
		input, err = getKeyValue(input, nod)
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
		return termValue(input, nod)
	}
	if nod.Type&cArrayType > 0 {
		if input, err = sliceArray(input, nod); err != nil {
			return nil, err
		}
		if nod.Type&cAgg > 0 {
			return getNodes(input, nod.Next)
		}
	}
	return getValue(input, nod.Next)
}

func wildScan(input []byte, nod *tNode) (result []byte, err error) {
	result = []byte{}
	separator := byte('[')
	for {
		input, err = getKeyValue(input, nod)
		if err != nil {
			return nil, err
		}
		var elem []byte
		skip := 0

		if nod.Type&cIsTerminal > 0 {
			// any field type matches
			elem, err = termValue(input, nod)
			skip = len(elem)
		} else {
			if skip, err = skipValue(input, 0); err != nil {
				return nil, err
			}
			switch input[0] {
			case '[': // array type -- aggregate fields
				if nod.Type&cArrayType > 0 {
					if elem, err = getNodes(input, nod.Next); err != nil {
						return nil, err
					}
					if len(elem) > 2 {
						elem = elem[1 : len(elem)-1]
					}
				}
			case '{': // object type -- process jsonpath query
				if nod.Type&cArrayType == 0 {
					if elem, err = getValue(input[:skip], nod.Next); err != nil {
						return nil, err
					}
				}
			}
		}
		if len(elem) > 0 {
			result = append(append(result, separator), elem...)
			separator = ','
		}
		input = input[skip:]

		i, err := skipSpaces(input, 0)
		if err != nil {
			return nil, err
		}
		if input[i] == '}' {
			break
		}
	}
	return append(result, ']'), nil
}

func termValue(input []byte, nod *tNode) ([]byte, error) {
	if nod.Type&cArrayType > 0 {
		return sliceArray(input, nod)
	}
	eoe, err := skipValue(input, 0)
	if err != nil {
		return nil, err
	}
	return input[:eoe], nil
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
func getKeyValue(input []byte, nod *tNode) ([]byte, error) {
	var (
		err error
		ch  byte
		s   int
		e   int
	)
	i := 1
	l := len(input)
	separator := byte('[')

	ret := []byte{}
	elems := make([][]byte, len(nod.Keys))

	for i < l && input[i] != '}' {
		state := keySeek
		for i < l && state != keyClose {
			ch = input[i]
			if ch == '"' {
				if state == keySeek {
					state = keyOpen
					s = i + 1
				} else if state == keyOpen {
					state = keyClose
					e = i
				}
			}
			i++
		}

		if state == keyClose {
			i, err = seekToValue(input, i)
			if err != nil {
				return nil, err
			}
			var hit bool
			hit, i, err = keyCheck(input[s:e], input, i, nod, elems)
			if hit || err != nil {
				return input[i:], err
			}
		}
	}
	if len(nod.Keys) > 0 {
		for i := 0; i < len(nod.Keys); i++ {
			if len(elems[i]) > 0 {
				ret = append(ret, separator)
				ret = append(ret, elems[i]...)
				separator = ','
			}
		}
		return append(ret, ']'), nil
	}
	return nil, errors.New(`field not found`)
}

func keyCheck(key []byte, input []byte, i int, nod *tNode, elems [][]byte) (bool, int, error) {
	var e int
	var err error

	if bytes.EqualFold(nod.Key, key) || (len(nod.Key) == 1 && nod.Key[0] == '*') {
		return true, i, nil // single key hit
	}

	s := i
	e, err = skipValue(input, i)
	if err != nil {
		return false, i, err
	}
	i, err = skipSpaces(input, e)
	if err != nil {
		return false, i, err
	}

	for ii, k := range nod.Keys {
		if bytes.EqualFold(k, key) {
			elems[ii] = input[s:e]
			return false, i, nil
		}
	}

	return false, i, nil
}

type tElem struct {
	start int
	end   int
}

// sliceArray select node(s) by bound(s)
func sliceArray(input []byte, nod *tNode) ([]byte, error) {
	if input[0] != '[' {
		return nil, errors.New("array expected")
	}
	i := 1 // skip '['

	if nod.Type&cArrayRanged == 0 && nod.Left >= 0 && len(nod.Elems) == 0 {
		// single positive index -- easiest case
		return getArrayElement(input, i, nod)
	}
	if nod.Filter != nil {
		// filtered array
		return getFilteredElements(input, i, nod)
	}

	// fullscan
	var elems []tElem
	var err error
	elems, err = arrayScan(input)
	if err != nil {
		return nil, err
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
		a := nod.Left + len(elems) // nod.Left is negative, so correct it to a real element index
		if a < 0 {
			return nil, errors.New("specified element not found")
		}
		return input[elems[a].start:elems[a].end], nil
	}
	// two bounds
	a, b, err := adjustBounds(nod.Left, nod.Right, len(elems))
	if err != nil {
		return nil, err
	}
	if len(elems) > 0 {
		input = input[elems[a].start:elems[b].end]
		input = input[:len(input):len(input)]
	} else {
		input = []byte{}
	}
	return append([]byte{'['}, append(input, ']')...), nil
}

func arrayScan(input []byte) ([]tElem, error) {
	l := len(input)
	elems := make([]tElem, 0, 32)
	// skip spaces before value
	i, err := skipSpaces(input, 1)
	if err != nil {
		return nil, err
	}
	for i < l && input[i] != ']' {
		e, err := skipValue(input, i)
		if err != nil {
			return nil, err
		}
		elems = append(elems, tElem{i, e})
		// skip spaces after value
		i, err = skipSpaces(input, e)
		if err != nil {
			return nil, err
		}
	}
	return elems, nil
}

func getArrayElement(input []byte, i int, nod *tNode) ([]byte, error) {
	var err error
	l := len(input)
	i, err = skipSpaces(input, i)
	if err != nil {
		return nil, err
	}
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

func getFilteredElements(input []byte, i int, nod *tNode) ([]byte, error) {
	l := len(input)
	result := []byte{'['}
	// fullscan
	for i < l && input[i] != ']' {
		e, err := skipValue(input, i)
		if err != nil {
			return nil, err
		}
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
		// skip spaces after value
		i, err = skipSpaces(input, e)
		if err != nil {
			return nil, err
		}
	}
	return append(result, ']'), nil
}

func adjustBounds(left int, right int, n int) (int, int, error) {
	a := left
	b := right
	if b == 0 {
		b = n
	}
	if a < 0 {
		a += n
	}
	if b < 0 {
		b += n
	}
	b-- // right bound excluded
	if n > 0 && (a < 0 || a >= n || b < 0 || b >= n) {
		return 0, 0, errors.New("specified element not found")
	}
	return a, b, nil
}

func seekToValue(input []byte, i int) (int, error) {
	var err error
	// spaces before ':'
	i, err = skipSpaces(input, i)
	if err != nil {
		return 0, err
	}
	if input[i] != ':' {
		return 0, errors.New("':' expected")
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
		return skipObject(input, i)
	} else {
		if (input[i] >= '0' && input[i] <= '9') || input[i] == '-' || input[i] == '.' {
			// number
			i = skipNumber(input, i)
		} else {
			// bool, null
			i, err = skipBoolNull(input, i)
			if err != nil {
				return i, err
			}
		}
	}
	return i, nil
}

func skipNumber(input []byte, i int) int {
	l := len(input)
	for ; i < l; i++ {
		ch := input[i]
		if !((ch >= '0' && ch <= '9') || ch == '.' || ch == '-' || ch == 'E' || ch == 'e') {
			break
		}
	}
	return i
}

func skipBoolNull(input []byte, i int) (int, error) {
	needles := [...][]byte{[]byte("true"), []byte("false"), []byte("null")}
	for n := 0; n < len(needles); n++ {
		if matchSubslice(input[i:], needles[n]) {
			return i + len(needles[n]), nil
		}
	}
	return i, errors.New("unrecognized value")
}

func matchSubslice(str, needle []byte) bool {
	l := len(needle)
	if len(str) < l {
		return false
	}
	for i := 0; i < l; i++ {
		if str[i] != needle[i] {
			return false
		}
	}
	return true
}

func checkValueType(input []byte, nod *tNode) error {
	if len(input) < 2 {
		return errors.New("unexpected end of input")
	}
	if nod.Type&cSubject > 0 {
		return nil
	}
	if nod.Type&cIsTerminal > 0 {
		return nil
	}
	ch := input[0]
	if nod.Type&cArrayType == 0 && ch != '{' {
		return errors.New("object expected")
	} else if nod.Type&cArrayType > 0 && ch != '[' {
		return errors.New("array expected")
	}
	return nil
}

func doFunc(input []byte, nod *tNode) ([]byte, error) {
	var err error
	var result int
	if bytes.Equal(word("size"), nod.Key) {
		result, err = skipValue(input, 0)
	} else if bytes.Equal(word("length"), nod.Key) || bytes.Equal(word("count"), nod.Key) {
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
		if !bytein(input[i], []byte{' ', ',', '\t', '\r', '\n'}) {
			break
		}
	}
	if i == l {
		return i, errors.New("unexpected end of input")
	}
	return i, nil
}

func skipString(input []byte, i int) (int, error) {
	bound := input[i]
	prev := bound
	done := false
	i++
	l := len(input)
	for i < l && !done {
		ch := input[i]
		if ch == bound && prev != '\\' {
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

func skipObject(input []byte, i int) (int, error) {
	l := len(input)
	mark := input[i]
	unmark := mark + 2 // ] or }
	nested := 0
	instr := false
	prev := mark
	i++
	for i < l && !(input[i] == unmark && nested == 0 && !instr) {
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
	return i, nil
}
