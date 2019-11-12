package jsonslice

/**
  JsonSlice 0.7.4
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

	errPathEmpty,
	errPathInvalidChar,
	errPathRootExpected,
	errPathUnexpectedEnd,
	errPathInvalidReference,
	errPathUnknownFunction,
	errPathIndexBoundMissing,
	errPathKeyListTerminated,
	errPathIndexNonsense,
	errArrayElementNotFound,
	errFieldNotFound,
	errArrayExpected,
	errColonExpected,
	errUnrecognizedValue,
	errUnexpectedEnd,
	errObjectExpected,
	errInvalidLengthUsage,
	errObjectOrArrayExpected,
	errWildcardsNotSupported,
	errFunctionsNotSupported,
	errTerminalNodeArray,
	errSubslicingNotSupported,
	errUnexpectedEOT,
	errUnknownToken,
	errUnexpectedStringEnd,
	errInvalidBoolean,
	errEmptyFilter,
	errNotEnoughArguments,
	errUnknownOperator,
	errInvalidArithmetic,
	errInvalidRegexp,
	errOperandTypes,
	errInvalidOperatorStrings error
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

	errPathEmpty = errors.New("path: empty")
	errPathInvalidChar = errors.New("path: invalid character")
	errPathRootExpected = errors.New("path: $ expected")
	errPathUnexpectedEnd = errors.New("path: unexpected end of path")
	errPathInvalidReference = errors.New("path: invalid element reference")
	errPathUnknownFunction = errors.New("path: unknown function")
	errPathIndexBoundMissing = errors.New("path: index bound missing")
	errPathKeyListTerminated = errors.New("path: key list terminated unexpectedly")
	errPathIndexNonsense = errors.New("path: 0 as a second bound does not make sense")
	errArrayElementNotFound = errors.New(`specified array element not found`)
	errFieldNotFound = errors.New(`field not found`)
	errColonExpected = errors.New("':' expected")
	errUnrecognizedValue = errors.New("unrecognized value: true, false or null expected")
	errUnexpectedEnd = errors.New("unexpected end of input")
	errObjectExpected = errors.New("object expected")
	errArrayExpected = errors.New("array expected")
	errInvalidLengthUsage = errors.New("length() is only applicable to array or string")
	errObjectOrArrayExpected = errors.New("object or array expected")
	errWildcardsNotSupported = errors.New("wildcards are not supported in GetArrayElements")
	errFunctionsNotSupported = errors.New("functions are not supported in GetArrayElements")
	errTerminalNodeArray = errors.New("terminal node must be an array")
	errSubslicingNotSupported = errors.New("sub-slicing is not supported in GetArrayElements")
	errUnexpectedEOT = errors.New("unexpected end of token")
	errUnknownToken = errors.New("unknown token")
	errUnexpectedStringEnd = errors.New("unexpected end of string")
	errInvalidBoolean = errors.New("invalid boolean value")
	errEmptyFilter = errors.New("empty filter")
	errNotEnoughArguments = errors.New("not enough arguments")
	errUnknownOperator = errors.New("unknown operator")
	errInvalidArithmetic = errors.New("invalid operands for arithmetic operator")
	errInvalidRegexp = errors.New("invalid operands for regexp match")
	errOperandTypes = errors.New("operand types do not match")
	errInvalidOperatorStrings = errors.New("operator is not applicable to strings")
}

type word []byte

type tNode struct {
	Key    word
	Keys   []word
	Type   int // properties
	Left   int // >=0 index from the start, <0 backward index from the end
	Right  int // 0 till the end inclusive, >0 to index exclusive, <0 backward index from the end exclusive
	Step   int //
	Elems  []int
	Next   *tNode
	Filter *tFilter
}

func getEmptyNode() *tNode {
	nod := nodePool.Get().(*tNode)
	nod.Elems = nod.Elems[:0]
	nod.Filter = nil
	nod.Key = nod.Key[:0]
	nod.Keys = nod.Keys[:0]
	nod.Left = cNAN
	nod.Right = cNAN
	nod.Step = cEmpty
	nod.Next = nil
	nod.Type = 0
	return nod
}

// Get returns a part of input, matching jsonpath.
// In terms of allocations there are two cases of retreiving data from the input:
// 1. (simple case) the result is a simple subslice of a source input.
// 2. the result is a merge of several non-contiguous parts of input. More allocations are needed.
func Get(input []byte, path string) ([]byte, error) {

	if len(path) == 0 {
		return nil, errPathEmpty
	}

	if len(path) == 1 && path[0] == '$' {
		return input, nil
	}

	if path[0] != '$' {
		return nil, errPathRootExpected
	}

	node, i, err := readRef(unspace([]byte(path)), 1)
	if err != nil {
		repool(node)
		return nil, errors.New(err.Error() + " at " + strconv.Itoa(i))
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

	result, err := getValue2(input, node)

	repool(node)
	return result, err
}

const (
	cDot        = 1 << iota // common [dot-]node
	cIsTerminal = 1 << iota // terminal node
	cAgg        = 1 << iota // aggregating
	cFunction   = 1 << iota // function
	cSlice      = 1 << iota // slice array [x:y:s]
	cFullScan   = 1 << iota // array slice: need fullscan
	cFilter     = 1 << iota // filter
	cWild       = 1 << iota // wildcard (*)
	cDeep       = 1 << iota // deepscan

	cSubject = 1 << iota // function subject
	cIndex   = 1 << iota // single index

	cCurrent = 1 << iota // key is referred to current node
	cRoot    = 1 << iota // key is referred to root

	cEmpty = 1 << 29 // empty number
	cNAN   = 1 << 30 // not-a-number
)

// returns true if b matches one of the elements of seq
func bytein(b byte, seq []byte) bool {
	for i := 0; i < len(seq); i++ {
		if b == seq[i] {
			return true
		}
	}
	return false
}

var keyTerminator = []byte{' ', '\t', ':', '.', '[', '(', ')', ']', '<', '=', '>', '+', '-', '*', '/', '&', '|'}

// parse jsonpath and return a root of a linked list of nodes
func parsePath(path []byte) (*tNode, int, error) {
	var err error
	var done bool
	var nod *tNode

	i := 0
	l := len(path)

	if l == 0 {
		return nil, i, errPathUnexpectedEnd
	}

	nod, i, err = readRef(path, i)
	if err != nil {
		return nil, i, err
	}

	// get key
	if path[i] == '*' {
		i++
		nod.Type &= cWild
	} else {
		for ; i < l && !bytein(path[i], keyTerminator); i++ {
		}
		nod.Key = path[:i]
	}

	if i == l {
		// finished parsing
		nod.Type |= cIsTerminal
		return nod, i, nil
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
		return nod, i, err
	}

	next, j, err := parsePath(path[i:])
	i += j
	if err != nil {
		return nil, i, err
	}
	nod.Next = next
	if next.Type&cFunction > 0 {
		nod.Type |= cSubject
	}
	return nod, i, nil
}

func readRef(path []byte, i int) (*tNode, int, error) {
	var err error
	var next *tNode
	var sep byte
	var flags int
	//var sep byte
	nod := getEmptyNode()

	l := len(path)
	// [optional] dots
	if path[i] == '.' {
		if i+1 < l && path[i+1] == '.' {
			nod.Type |= cDeep
			i++
		} else {
			nod.Type |= cDot
		}
		i++
	}
	if i == l {
		return nil, i, errUnexpectedEnd
	}
	// bracket notated
	if path[i] == '[' {
		i++
		i, err = readBrackets(nod, path, i)
		if i == l || err != nil {
			return nod, i, err
		}
		next, i, err = readRef(path, i)
		nod.Next = next
		return nod, i, err
	}
	if nod.Type&(cDot|cDeep) == 0 {
		return nil, i, errPathInvalidChar
	}
	// single key
	nod.Key, nod.Left, sep, i, flags, err = readKey(path, i)
	nod.Type |= flags // cWild, cFullScan
	if i == l {
		return nod, i, nil
	}
	// function
	if sep == '(' && i+1 < l && path[i+1] == ')' {
		_, i, err = detectFn(path, i, nod)
		return nod, i, err
	}
	// path end?
	if bytein(sep, pathTerminator) {
		return nod, i, nil
	}

	// recursive
	next, i, err = readRef(path, i)
	nod.Next = next
	return nod, i, err
}

// read bracket-notated keys
// consumes final ']'
func readBrackets(nod *tNode, path []byte, i int) (int, error) {
	var (
		key   []byte
		ikey  int
		sep   byte
		err   error
		flags int
	)
	l := len(path)
	if i < l-1 && path[i] == '?' && path[i+1] == '(' {
		return readFilter(path, i+2, nod)
	}
	for pos := 0; i < l && path[i] != ']'; pos++ {
		key, ikey, sep, i, flags, err = readKey(path, i)
		nod.Type |= flags // cWild, cFullScan // CAUTION: [*,1,2] possible
		if err != nil {
			return i, err
		}
		err = setKey(nod, key, ikey, sep, pos)
		if err != nil {
			return i, err
		}
		if nod.Type&(cSlice|cAgg) == cSlice|cAgg {
			return i, errPathInvalidChar
		}
		if sep == ':' || sep == ',' {
			i++
		}
	}
	if i == l {
		return i, errPathUnexpectedEnd
	}
	if nod.Type&cSlice > 0 && nod.Left+nod.Right+nod.Step == cEmpty*3 {
		nod.Type |= cWild
	}
	i++ // ']'
	return i, nil
}

// read next key
// targeted to: ".key", ".*", ".1", ".-1"
// returns:
//   key   = the key
//   ikey  = integer converted key
//   sep   = key list separator (expected , : [ ] . 0)
//   i     = current i (after the sep)
//   flags = cWild if wildcard
//   err   = error
func readKey(path []byte, i int) ([]byte, int, byte, int, int, error) {
	l := len(path)
	s := i
	if i == l {
		return nil, 0, 0, i, 0, errPathUnexpectedEnd
	}
	step := 0
	bound := byte(0)
	if bytein(path[i], []byte{'\'', '"'}) {
		bound = path[i]
		step++
		i++
	}
	prev := byte(0)
	if bound > 0 {
		for i < l {
			if prev != '\\' && path[i] == bound {
				break
			}
			prev = path[i]
			i++
		}
	} else {
		if path[i] == '-' {
			i++
		}
		for i < l {
			if bytein(path[i], keyTerminator) {
				break
			}
			prev = path[i]
			i++
		}
	}
	if i != l || bound == 0 {
		return path[s:i], toInt(path[s:i]), path[i], i + step, flags(i-s, path[s], toInt(path[s:i])), nil
	}
	return nil, cNAN, 0, i, 0, errPathUnexpectedEnd
}

func flags(n int, ch byte, ikey int) int {
	flag := 0
	if ikey < 0 || ikey == cEmpty {
		flag |= cFullScan
	}
	if n == 1 && ch == '*' {
		flag |= cWild
	}
	return flag
}

func toInt(buf []byte) int {
	if len(buf) == 0 {
		return cEmpty
	}
	n, err := strconv.Atoi(string(buf))
	if err != nil {
		return cNAN
	}
	return n
}

func setKey(nod *tNode, key []byte, ikey int, sep byte, pos int) error {
	switch sep {
	case ']':
		// end of key list
	case ',':
		nod.Type |= cAgg | cDot // cDot to extract values
	case ':':
		nod.Type |= cSlice
	case 0:
		return errPathUnexpectedEnd
	default:
		return errPathInvalidChar
	}
	if nod.Type&cAgg > 0 {
		nod.Keys = append(nod.Keys, key)
		if ikey != cNAN {
			nod.Elems = append(nod.Elems, ikey)
		}
	} else if nod.Type&cSlice > 0 {
		if ikey == cNAN {
			return errPathInvalidChar
		}
		switch pos {
		case 0:
			nod.Left = ikey
		case 1:
			nod.Right = ikey
		case 2:
			nod.Step = ikey
		default:
			return errPathInvalidChar
		}
	} else {
		nod.Key = key
		nod.Left = ikey
		nod.Type |= cDot
	}
	return nil
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
			return true, i, errPathUnexpectedEnd
		}
		if path[i] == '.' {
			nod.Type |= cDeep
			i++
		}
	} else if ch != '[' { // nested array
		return true, i, errPathInvalidReference
	}
	return false, i, nil
}

func detectFn(path []byte, i int, nod *tNode) (bool, int, error) {
	if !(bytes.EqualFold(nod.Key, []byte("length")) ||
		bytes.EqualFold(nod.Key, []byte("count")) ||
		bytes.EqualFold(nod.Key, []byte("size"))) {
		return true, i, errPathUnknownFunction
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
	if i < l-1 && path[i] == '?' && path[i+1] == '(' {
		// filter
		nod.Type |= cFilter | cAgg
		i, err = readFilter(path, i+2, nod)
		if err != nil {
			return i, err
		}
		i++ // )
	} else {
		// single index, slice or index list
		i, err = readArrayKeys(path, i, nod)
		if err != nil {
			return i, err
		}
	}
	if i >= l || path[i] != ']' {
		return i, errPathIndexBoundMissing
	}
	i++ // ]
	return i, nil
}

const modeUnknown int = 0
const modeSlice int = 1
const modeKeys int = 2

func readArrayKeys(path []byte, i int, nod *tNode) (int, error) {

	l := len(path)

	mode := modeUnknown
	var (
		err     error
		key     []byte
		m, ikey int
	)

	part := 0
	for i < l && path[i] != ']' {
		i, key, ikey, m, err = getKey(path, i, mode)
		if err != nil {
			return i, err
		}
		if mode != modeUnknown && mode != m {
			return i, errPathInvalidReference
		}
		mode = m
		if mode == modeKeys {
			if part > 0 {
				nod.Type |= cAgg
			}
			// keys list
			nod.Keys = append(nod.Keys, key)
			if ikey != cNAN {
				nod.Elems = append(nod.Elems, ikey)
			}
		} else {
			// slice bounds
			switch part {
			case 0:
				nod.Left = ikey
				nod.Type |= cSlice
			case 1:
				nod.Right = ikey
			case 2:
				nod.Step = ikey
			default:
				return i, errPathInvalidReference
			}
		}
		part++
	}
	return i, err
}

func getKey(path []byte, i int, mode int) (int, []byte, int, int, error) {
	bound := byte(0)
	l := len(path)
	a := i
	if i < l && bytein(path[i], []byte{'\'', '"'}) {
		bound = path[i]
		mode = modeKeys
		i++
		a++
	}
	for i < l && !(path[i] == bound || (bound == 0 && bytein(path[i], []byte{' ', ':', ',', ']'}))) {
		i++
	}
	b := i
	n, err := strconv.Atoi(string(path[a:i]))
	if err != nil {
		n = cNAN
	}
	if i < l && path[i] == bound {
		i++
	}
	for i < l && bytein(path[i], []byte{' ', ','}) {
		mode = modeKeys
		i++
	}
	if i == l {
		return i, nil, 0, mode, errPathUnexpectedEnd
	}
	if path[i] == ':' {
		mode = modeSlice
		i++
	}
	return i, path[a:b], n, mode, nil
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
			return i, errPathKeyListTerminated
		}
		nod.Keys = append(nod.Keys, path[i:e])
		i = e + 1 // skip '
		for ; i < l && path[i] != '\'' && path[i] != ']'; i++ {
		} // sek to next ' or ]
	}
	if i == l {
		return i, errPathKeyListTerminated
	}
	i++ // ]
	if i == l {
		nod.Type |= cIsTerminal
	}
	return i, nil
}

/*
func readArrayIndex(path []byte, i int, nod *tNode) (int, error) {
	l := len(path)
	num := 0
	num, i = readInt(path, i)
	if i == l || !bytein(path[i], []byte{':', ',', ']'}) {
		return i, errPathIndexBoundMissing
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
			return i, errPathIndexNonsense
		}
		i = ii
		nod.Right = num
	}
	return i, nil
}
*/

func getValue(input []byte, nod *tNode) (result []byte, err error) {

	i, _ := skipSpaces(input, 0)

	input = input[i:]
	if err = looksLikeJSON(input); err != nil {
		return nil, err
	}
	// wildcard
	if nod.Type&cWild > 0 {
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
	if nod.Type&cFilter > 0 {
		return getFilteredElements(input, nod)
	}
	if input, err = slice(input, nod); err != nil {
		return nil, err
	}
	if nod.Type&cAgg > 0 {
		return getNodes(input, nod.Next)
	}

	return getValue(input, nod.Next)
}

func getValue2(input []byte, nod *tNode) (result []byte, err error) {

	if nod == nil {
		return input, nil
	}
	if len(input) == 0 {
		return input, nil
	}

	i, _ := skipSpaces(input, 0) // we're at the value
	input = input[i:]

	switch {
	case nod.Type&cDot > 0: // single or multiple key
		result, err = getValueDot(input, nod) // recurse inside
	case nod.Type&cSlice > 0: // array slice
		result, err = getValueSlice(input, nod) // recurse inside
	case nod.Type&cFunction > 0: // func()
		result, err = doFunc(input, nod) // no recurse
	default:
		return nil, errFieldNotFound
	}
	if nod.Type&(cAgg|cSlice) > 0 {
		result = append(append([]byte{'['}, result...), byte(']'))
	}
	return result, err
}

// *** : $.foo, $['foo','bar'], $[1], $[1,2]
func getValueDot(input []byte, nod *tNode) (result []byte, err error) {
	if len(input) == 0 {
		return
	}
	switch input[0] {
	case '{':
		return objectValueByKey(input, nod) // 1+ (recurse inside)
	case '[':
		return arrayElemByIndex(input, nod) // 1+ (recurse inside)
	default:
		return nil, errObjectOrArrayExpected
	}
}

// *** : $[1:3], $[1:7:2]
func getValueSlice(input []byte, nod *tNode) (result []byte, err error) {
	if len(input) == 0 {
		return
	}
	switch input[0] {
	case '{':
		return
	case '[':
		return arraySlice(input, nod) // 1+ (recurse inside)
	default:
		return nil, errObjectOrArrayExpected
	}
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
				if elem, err = getNodes(input, nod.Next); err != nil {
					return nil, err
				}
				if len(elem) > 2 {
					elem = elem[1 : len(elem)-1]
				}
			case '{': // object type -- process jsonpath query
				if elem, err = getValue(input[:skip], nod.Next); err != nil {
					return nil, err
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
	if input[0] == '[' {
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
	if len(elems) > 0 {
		for i := 0; i < len(elems); i++ {
			ret = append(ret, separator)
			ret = append(ret, elems[i]...)
			separator = ','
		}
		return append(ret, ']'), nil
	}
	return nil, errArrayElementNotFound
}

// ***
func objectValueByKey(input []byte, nod *tNode) ([]byte, error) {
	var (
		err error
		key []byte
	)
	i := 1 // skip '{'
	l := len(input)
	var ret []byte
	var elems [][]byte
	if len(nod.Keys) > 0 {
		elems = make([][]byte, len(nod.Keys))
	}

	for i < l && input[i] != '}' {
		key, i, err = readObjectKey(input, i)
		var hit bool
		hit, i, err = keyCheck(key, input, i, nod, elems)
		if hit {
			return getValue2(input[i:], nod.Next)
		}
		if err != nil {
			return nil, err
		}
	}
	if i == l {
		return nil, errUnexpectedEnd
	}
	for i := 0; i < len(elems); i++ {
		if elems[i] != nil {
			sub, err := getValue2(elems[i], nod.Next)
			if err != nil {
				return nil, err
			}
			if len(sub) > 0 {
				if len(ret) > 0 {
					ret = append(ret, ',')
				}
				ret = append(ret, sub...)
			}
		}
	}
	return ret, nil
}

// [x]
// seek to key
// read key
// seek to value
//
// return key, i
//
func readObjectKey(input []byte, i int) ([]byte, int, error) {
	l := len(input)
	for input[i] != '"' {
		i++
		if i == l {
			return nil, i, errUnexpectedEnd
		}
		if input[i] == '}' {
			return nil, i, nil
		}
	}
	s := i
	e, err := skipString(input, i)
	if err != nil {
		return nil, i, err
	}
	i, err = seekToValue(input, e)
	if err != nil {
		return nil, i, err
	}
	return input[s:e], i, nil
}

// ***
// get array element(s) by index
// $[3] or $[1,2,-3]
// recurse inside
//
func arrayElemByIndex(input []byte, nod *tNode) ([]byte, error) {
	elems, elem, err := arrayIterateElems(input, nod)
	if err != nil {
		return nil, err
	}
	if elem != nil {
		return getValue2(elem, nod.Next)
	}
	return collectRecurse(input, nod, elems)
}

func arraySlice(input []byte, nod *tNode) ([]byte, error) {
	elems, _, err := arrayIterateElems(input, nod)
	if err != nil {
		return nil, err
	}
	return sliceRecurse(input, nod, elems)
}

func arrayIterateElems(input []byte, nod *tNode) (elems []tElem, elem []byte, err error) {
	var i, s, e int
	l := len(input)
	if nod.Type&(cFullScan|cWild) > 0 {
		elems = make([]tElem, 0, 32)
	}
	// skip spaces before value
	i, err = skipSpaces(input, 1) // skip '['
	if err != nil {
		return
	}
	for pos := 0; i < l && input[i] != ']'; {
		s = i
		e, err = skipValue(input, i)
		if err != nil {
			return
		}
		// skip spaces after value
		i, err = skipSpaces(input, e)
		if err != nil {
			return
		}
		found := nod.Type&(cFullScan|cWild) > 0
		for f := 0; !found && f < len(nod.Elems); f++ {
			if nod.Elems[f] == pos {
				found = true
			}
		}
		if nod.Left == pos {
			elem = input[s:e]
			return
		}
		if found {
			elems = append(elems, tElem{s, e})
		}
	}
	return
}

// ***
func collectRecurse(input []byte, nod *tNode, elems []tElem) ([]byte, error) {
	var res []byte
	var err error
	// collect & recurse
	sep := []byte(nil)
	for i := 0; i < len(nod.Elems); i++ {
		e := nod.Elems[i]
		if e < 0 {
			e += len(nod.Elems)
		}
		if e < len(nod.Elems) {
			sub, err := getValue2(input[elems[i].start:elems[i].end], nod.Next)
			if err != nil {
				return nil, err
			}
			if len(sub) > 0 {
				res = append(append(res, sep...), sub...)
				sep = []byte{','}
			}
		}
	}
	return res, err
}

// ***
func sliceRecurse(input []byte, nod *tNode, elems []tElem) ([]byte, error) {
	var res []byte
	var sub []byte
	var err error
	a, b, step, err := adjustBounds(nod.Left, nod.Right, nod.Step, len(elems))
	if err != nil {
		return nil, err
	}
	sep := []byte(nil)
	// collect & recurse
	for ; (a > b && step < 0) || (a < b && step > 0); a += step {
		sub, err = getValue2(input[elems[a].start:elems[a].end], nod.Next)
		if err != nil {
			return nil, err
		}
		if len(sub) > 0 {
			res = append(append(res, sep...), sub...)
			sep = []byte{','}
		}
	}
	return res, err
}

/*
	input:
	 - key		= key that has been found
	 - input	= input[i]
	 - i		= points at value
	 - nod		= current node
	 - elems	=
	return:
	 - hit	    = single key hit
	 - i        =
	 - error    =
*/
func keyCheck(key []byte, input []byte, i int, nod *tNode, elems [][]byte) (bool, int, error) {
	var e int
	var err error

	if bytes.EqualFold(nod.Key, key) {
		return true, i, nil // single key hit
	}

	s := i
	e, err = skipValue(input, i) // s:e holds a value
	if err != nil {
		return false, i, err
	}
	i, err = skipSpaces(input, e)
	if err != nil {
		return false, i, err
	}

	for ii, k := range nod.Keys {
		if nod.Type&cWild > 0 || bytes.EqualFold(k, key) {
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
func slice(input []byte, nod *tNode) ([]byte, error) {

	if input[0] != '[' {
		return nil, errArrayExpected
	}
	i := 1 // skip '['

	if nod.Type&cSlice == 0 && nod.Left >= 0 && len(nod.Elems) == 0 {
		// single positive index -- easiest case
		return getArrayElement(input, i, nod)
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
	if nod.Type&cIndex > 0 {
		a := nod.Left + len(elems) // nod.Left is negative, so correct it to a real element index
		if a < 0 {
			return nil, errArrayElementNotFound
		}
		return input[elems[a].start:elems[a].end], nil
	}
	// two bounds
	a, b, _, err := adjustBounds(nod.Left, nod.Right, 1, len(elems))
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

// sliceArray select node(s) by bound(s)
func sliceArray(input []byte, nod *tNode) ([]byte, error) {
	if input[0] != '[' {
		return nil, errArrayExpected
	}
	i := 1 // skip '['

	if nod.Type&cSlice == 0 && nod.Left >= 0 && len(nod.Elems) == 0 {
		// single positive index -- easiest case
		return getArrayElement(input, i, nod)
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
	if nod.Type&cIndex > 0 {
		a := nod.Left + len(elems) // nod.Left is negative, so correct it to a real element index
		if a < 0 {
			return nil, errArrayElementNotFound
		}
		return input[elems[a].start:elems[a].end], nil
	}
	// two bounds
	a, b, _, err := adjustBounds(nod.Left, nod.Right, 1, len(elems))
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
	return nil, errArrayElementNotFound
}

func getFilteredElements(input []byte, nod *tNode) ([]byte, error) {
	if len(input) > 0 && input[0] != '[' {
		return nil, errArrayExpected
	}
	i := 1 // skip '['
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

func adjustBounds(start int, stop int, step int, n int) (int, int, int, error) {
	if step == 0 || step == cEmpty {
		step = 1
	}
	if start == cEmpty {
		if step > 0 {
			start = 0
		} else {
			start = n - 1
		}
	}
	if stop < 0 {
		stop += n
	}
	if stop < 0 {
		stop = -1
	}
	if stop == cEmpty {
		if step > 0 {
			stop = n
		} else {
			stop = -1
		}
	}
	if start < 0 {
		start += n
	}
	if start < 0 {
		start = 0
	}
	if start >= n {
		start = n - 1
	}
	if stop > n {
		stop = n
	}
	return start, stop, step, nil
}

func seekToValue(input []byte, i int) (int, error) {
	var err error
	// spaces before ':'
	i, err = skipSpaces(input, i)
	if err != nil {
		return 0, err
	}
	if input[i] != ':' {
		return 0, errColonExpected
	}
	i++ // colon
	return skipSpaces(input, i)
}

// skips value, return (i) position of the 1st char after the value
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
	return i, errUnrecognizedValue
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
		return errUnexpectedEnd
	}
	if nod.Type&cSubject > 0 {
		return nil
	}
	if nod.Type&cIsTerminal > 0 {
		return nil
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
			return nil, errInvalidLengthUsage
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
		return i, errUnexpectedEnd
	}
	return i, nil
}

func skipSp(input []byte, i int) (int, byte) {
	l := len(input)
	for ; i < l && (input[i] == ' ' || input[i] == '\t'); i++ {
	}
	if i == l {
		return i, 0
	}
	return i, input[i]
}

// *** : skip quoted string (consumes last bound)
func skipString(input []byte, i int) (int, error) {
	bound := input[i]
	done := false
	escaped := false
	i++ // bound
	l := len(input)
	for i < l && !done {
		ch := input[i]
		if ch == bound && !escaped {
			done = true
		}
		escaped = ch == '\\' && !escaped
		i++
	}
	if i == l && !done {
		return 0, errUnexpectedEnd
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
		return 0, errUnexpectedEnd
	}
	i++ // closing mark
	return i, nil
}

func repool(node *tNode) {
	// return nodes back to pool
	for {
		if node == nil {
			break
		}
		p := node.Next
		nodePool.Put(node)
		node = p
	}
}

func looksLikeJSON(input []byte) error {
	if len(input) == 0 {
		return errUnexpectedEnd
	}
	if input[0] != '{' && input[0] != '[' {
		return errObjectOrArrayExpected
	}
	return nil
}

func unspace(buf []byte) []byte {
	r, w := 0, 0
	bound := byte(0)
	for r < len(buf) {
		if (buf[r] == '\'' || buf[r] == '"') && bound == 0 {
			bound = buf[r]
		} else if buf[r] == bound {
			bound = 0
		}
		if (buf[r] != ' ' && buf[r] != '\t') || bound > 0 {
			if w != r {
				buf[w] = buf[r]
			}
			w++
		}
		r++
	}
	return buf[:w]
}
