package jsonslice

/**
  JsonSlice 1.0.1
  Michael Gurov, 2018-2019
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

import (
	"bytes"
	"encoding/hex"
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
	errPathUnknownFunction,
	errFieldNotFound,
	errColonExpected,
	errUnrecognizedValue,
	errUnexpectedEnd,
	errInvalidLengthUsage,
	errObjectOrArrayExpected,
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

var speChars [128]byte

func init() {

	speChars['b'] = '\b'
	speChars['f'] = '\f'
	speChars['n'] = '\n'
	speChars['r'] = '\r'
	speChars['t'] = '\t'

	nodePool = sync.Pool{
		New: func() interface{} {
			return &tNode{
				Keys:  make([]word, 0, 1), // most common case: a single key
				Elems: make([]int, 0),
			}
		},
	}

	errPathEmpty = errors.New("path: empty")
	errPathInvalidChar = errors.New("path: invalid character")
	errPathRootExpected = errors.New("path: $ expected")
	errPathUnexpectedEnd = errors.New("path: unexpected end of path")
	errPathUnknownFunction = errors.New("path: unknown function")
	errFieldNotFound = errors.New(`field not found`)
	errColonExpected = errors.New("':' expected")
	errUnrecognizedValue = errors.New("unrecognized value: true, false or null expected")
	errUnexpectedEnd = errors.New("unexpected end of input")
	errInvalidLengthUsage = errors.New("length() is only applicable to array or string")
	errObjectOrArrayExpected = errors.New("object or array expected")
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

const (
	cDot      = 1 << iota // common [dot-]node
	cAgg      = 1 << iota // aggregating
	cFunction = 1 << iota // function
	cSlice    = 1 << iota // slice array [x:y:s]
	cFullScan = 1 << iota // array slice: need fullscan
	cFilter   = 1 << iota // filter
	cWild     = 1 << iota // wildcard (*)
	cDeep     = 1 << iota // deepscan (..)

	cRoot = 1 << iota // key is referred from root

	cEmpty = 1 << 29 // empty number
	cNAN   = 1 << 30 // not-a-number
)

type tNode struct {
	//Key    word
	Keys   []word
	Type   int // properties
	Slice  [3]int
	Elems  []int
	Next   *tNode
	Filter *tFilter
}

func getEmptyNode() *tNode {
	nod := nodePool.Get().(*tNode)
	nod.Elems = nod.Elems[:0]
	nod.Filter = nil
	nod.Keys = nod.Keys[:0]
	nod.Slice[0] = cEmpty
	nod.Slice[1] = cEmpty
	nod.Slice[2] = 1
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

	node, i, err := readRef(unspace([]byte(path)), 1, 0)
	if err != nil {
		repool(node)
		return nil, errors.New(err.Error() + " at " + strconv.Itoa(i))
	}

	n := node
	for {
		if n == nil {
			break
		}
		if n.Filter != nil {
			for _, tok := range n.Filter.toks {
				if tok.Operand != nil && tok.Operand.Node != nil && tok.Operand.Node.Type&cRoot > 0 {
					val, err := getValue(input, tok.Operand.Node, false)
					if err != nil {
						// not found or other error
						tok.Operand.Type = cOpNull
					}
					decodeValue(val, tok.Operand)
					tok.Operand.Node = nil
				}
			}
		}
		n = n.Next
	}

	result, err := getValue(input, node, false)
	repool(node)
	return result, err
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

var keyTerminator = []byte{' ', '\t', ':', '.', ',', '[', '(', ')', ']', '<', '=', '>', '+', '-', '*', '/', '&', '|', '!'}

func readRef(path []byte, i int, uptype int) (*tNode, int, error) {
	var err error
	var next *tNode
	var sep byte
	var flags int
	var key word

	if i >= len(path) {
		return nil, i, nil
	}

	// path end?
	if bytein(path[i], pathTerminator) {
		return nil, i, nil
	}
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
		return nil, i, errPathUnexpectedEnd
	}
	// bracket notated
	if path[i] == '[' {
		i++
		i, err = readBrackets(nod, path, i)
		if i == l || err != nil {
			return nod, i, err
		}
		next, i, err = readRef(path, i, nod.Type)
		nod.Next = next
		return nod, i, err
	}
	if nod.Type&(cDot|cDeep) == 0 {
		return nil, i, errPathInvalidChar
	}
	// single key
	key, nod.Slice[0], sep, i, flags, err = readKey(path, i)
	if len(key) > 0 {
		nod.Keys = append(nod.Keys, key)
	}
	nod.Type |= flags // cWild, cFullScan
	if i == l {
		return nod, i, nil
	}
	// function
	if sep == '(' && i+1 < l && path[i+1] == ')' {
		_, i, err = detectFn(path, i, nod)
		return nod, i, err
	}

	// recursive
	next, i, err = readRef(path, i, nod.Type)
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
		nod.Type |= flags // cWild, cFullScan // CAUTION: [*,1,2] is possible
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
	if nod.Type&cSlice > 0 && nod.Slice[0]+nod.Slice[1]+nod.Slice[2] == 2*cEmpty+1 {
		nod.Type |= cWild
	}
	if len(nod.Elems) > 0 {
		nod.Type &^= cWild
	}
	i++ // ']'
	return i, nil
}

// read next key
// targeted to: ".key", ".*", ".1", ".-1"
// returns:
//   key   = the key
//   ikey  = integer converted key
//   sep   = key list separator (expected , : [ ] . +-*/=! 0)
//   i     = current i (after the sep)
//   flags = cWild if wildcard
//   err   = error
func readKey(path []byte, i int) ([]byte, int, byte, int, int, error) {
	l := len(path)
	s := i
	e := i
	if i == l {
		return nil, 0, 0, i, 0, errPathUnexpectedEnd
	}
	step := 0
	bound := byte(0)
	if bytein(path[i], []byte{'\'', '"'}) {
		bound = path[i]
		step++ // will be a closing bound
		s++    // start 1 char right
		i++
	}
	//prev := byte(0)
	if bound > 0 {
		s, e, i = readQuotedKey(path, i, bound)
	} else {
		if path[i] == '-' {
			i++
		}
		for i < l {
			if bytein(path[i], keyTerminator) {
				if path[i] == '*' && s-i == 0 {
					step++ // * usually * is a terminator but not in .* so skip it
				}
				break
			}
			i++
		}
		e = i
	}
	/*
		i!=l bound=0  // (unbounded) terminator reached s:i (empty for * -> step)
		i!=l bound="" // (bounded) key s:i (i==bound) -> step
		i==l bound=0  // (unbounded) EOL reached s:i, i==l
		i==l bound="" // (bounded) right bound missing
	*/
	if i == l && bound != 0 {
		// unclosed bound
		return nil, cNAN, 0, i, 0, errPathUnexpectedEnd
	}
	if i+step != l {
		bound = path[i+step]
	} else {
		bound = 0
	}
	n := toInt(path[s:e])
	return path[s:e], n, bound, i + step, flags(e-s, path[s], n), nil
}

// read quotedkey
// return
//   s start index
//   e end index
//   i current pointer
//
func readQuotedKey(path []byte, i int, bound byte) (int, int, int) {
	l := len(path)
	s := i
	w := i
	var prev byte

	for i < l {
		if path[i] == bound {
			if prev == '\\' {
				w--
			} else {
				break
			}
		}
		if i != w {
			path[w] = path[i]
		}
		prev = path[i]
		i++
		w++
	}
	return s, w, i
}

// flags determined by ikey
func flags(n int, ch byte, ikey int) int {
	flag := 0
	if ikey < 0 || ikey == cEmpty {
		flag |= cFullScan // fullscan if $[-1], $[1,-1] or $[1:-1] or $[1:]
	}
	if n == 0 && ch == '*' {
		flag |= cWild
	}
	return flag
}

func toInt(buf []byte) int {
	if len(buf) == 0 {
		return cEmpty
	}
	n := 0
	sign := 1
	for _, ch := range buf {
		if ch == '-' {
			sign = -1
			continue
		}
		if ch >= '0' && ch <= '9' {
			n = n*10 + int(ch-'0')
		} else {
			return cNAN
		}
	}
	return n * sign
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
		if ikey != cNAN && ikey != cEmpty {
			nod.Elems = append(nod.Elems, ikey)
		}
		return nil
	}
	if nod.Type&cSlice > 0 {
		if ikey == cNAN || pos > 2 {
			return errPathInvalidChar
		}
		if pos == 2 {
			if ikey == cEmpty || ikey == 0 {
				ikey = 1
			}
			if ikey != 1 {
				nod.Type |= cFullScan
			}
		}
		nod.Slice[pos] = ikey
		return nil
	}
	// cDot
	if len(key) > 0 {
		nod.Keys = append(nod.Keys, key)
	}
	nod.Slice[0] = ikey
	nod.Type |= cDot
	return nil
}

var pathTerminator = []byte{' ', '\t', '<', '=', '>', '+', '-', '*', '/', ')', '&', '|', '!'}

func detectFn(path []byte, i int, nod *tNode) (bool, int, error) {
	if len(nod.Keys) == 0 {
		return true, i, errPathUnknownFunction
	}
	if !(bytes.EqualFold(nod.Keys[0], []byte("length")) ||
		bytes.EqualFold(nod.Keys[0], []byte("count")) ||
		bytes.EqualFold(nod.Keys[0], []byte("size"))) {
		return true, i, errPathUnknownFunction
	}
	nod.Type |= cFunction
	nod.Type &^= cDot
	return true, i + 2, nil
}

func getValue(input []byte, nod *tNode, inside bool) (result []byte, err error) {

	if len(input) == 0 {
		return nil, nil
	}
	if nod == nil {
		e, err := skipValue(input, 0)
		if !inside {
			return input[:e], err
		}
		return input[:e:e], err
	}
	i, _ := skipSpaces(input, 0) // we're at the value
	input = input[i:]

	agg := nod.Type&(cAgg|cSlice|cDeep|cWild|cFilter) > 0
	switch {
	case nod.Type&(cDot|cDeep) > 0: // single or multiple key
		result, err = getValueDot(input, nod, agg || inside) // recurse inside
	case nod.Type&cSlice > 0: // array slice [::]
		result, err = getValueSlice(input, nod) // recurse inside
	case nod.Type&cFunction > 0: // func()
		result, err = doFunc(input, nod) // no recurse
	case nod.Type&cFilter > 0: // [?(...)]
		result, err = getValueFilter(input, nod, agg || inside) // no recurse
	default:
		return nil, errFieldNotFound
	}
	if agg && !inside {
		result = append(append([]byte{'['}, result...), byte(']'))
	}
	return result, err
}

// $.foo, $['foo','bar'], $[1], $[1,2]
func getValueDot(input []byte, nod *tNode, inside bool) (result []byte, err error) {
	if len(input) == 0 {
		return
	}
	switch input[0] {
	case '{':
		return objectValueByKey(input, nod, inside) // 1+ (recurse inside) (+deep)
	case '[':
		return arrayElemByIndex(input, nod, inside) // 1+ (recurse inside)
	default:
		return nil, nil
	}
}

// $[1:3], $[1:7:2]
// $..[1:3], $..[1:7:2]
func getValueSlice(input []byte, nod *tNode) (result []byte, err error) {
	if len(input) == 0 {
		return
	}
	switch input[0] {
	case '{':
		if nod.Type&cDeep > 0 {
			return objectDeep(input, nod) // (recurse inside) (+deep)
		}
		return
	case '[':
		return arraySlice(input, nod) // 1+ (recurse inside) (+deep)
	default:
		return nil, nil
	}
}

func getValueFilter(input []byte, nod *tNode, inside bool) ([]byte, error) {
	if len(input) == 0 {
		return nil, errUnexpectedEnd
	}
	switch input[0] {
	case '{':
		if nod.Type&cDeep > 0 {
			return objectDeep(input, nod) // (recurse inside) (+deep)
		}
		return nil, nil
	case '[':
		return arrayElemByFilter(input, nod, true) // 1+ (recurse inside)
	default:
		return nil, errObjectOrArrayExpected
	}
}

// TODO: deep
func arrayElemByFilter(input []byte, nod *tNode, inside bool) (result []byte, err error) {
	var s, e int
	var b bool
	var sub []byte
	i := 1 // skip '['
	l := len(input)

	for i < l && input[i] != ']' {
		s, e, i, err = valuate(input, i)
		if err != nil {
			return nil, err
		}
		b, err = filterMatch(input[s:e], nod.Filter.toks)
		if err != nil {
			return nil, err
		}
		if b {
			sub, err = getValue(input[s:e], nod.Next, inside) // recurse
			if len(sub) > 0 {
				result = plus(result, sub)
			}
		}
	}
	return result, err
}

// ***
func objectValueByKey(input []byte, nod *tNode, inside bool) ([]byte, error) {
	var (
		err error
		key []byte
	)
	i := 1 // skip '{'
	l := len(input)
	var res []byte
	var elems [][]byte
	if len(nod.Keys) > 1 || nod.Type&cDeep > 0 {
		elems = make([][]byte, len(nod.Keys))
	}

	for i < l && input[i] != '}' {
		key, i, err = readObjectKey(input, i)
		if err != nil {
			return nil, err
		}
		elems, res, i, err = keyCheck(key, input, i, nod, elems, res, inside)
		if err != nil {
			return nil, err
		}
		if nod.Type&cDot > 0 && len(res) > 0 {
			return res, nil
		}
	}
	if i == l {
		return nil, errUnexpectedEnd
	}
	for i := 0; i < len(elems); i++ {
		if elems[i] != nil {
			res = plus(res, elems[i])
		}
	}
	return res, nil
}

func objectDeep(input []byte, nod *tNode) ([]byte, error) {
	var (
		err  error
		s, e int
		deep []byte
	)
	i := 1 // skip '{'
	l := len(input)
	var res []byte

	for i < l && input[i] != '}' {
		_, i, err = readObjectKey(input, i)
		if err != nil {
			return nil, err
		}
		s, e, i, err = valuate(input, i) // s:e holds a value
		if err != nil {
			return nil, err
		}
		deep, err = getValue(input[s:e], nod, true) // recurse
		if err != nil {
			return nil, err
		}
		if len(deep) > 0 {
			res = plus(res, deep)
		}
	}
	if i == l {
		return nil, errUnexpectedEnd
	}
	return res, nil
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
	s := i + 1
	e, err := skipString(input, i)
	if err != nil {
		return nil, i, err
	}
	i, err = seekToValue(input, e)
	if err != nil {
		return nil, i, err
	}
	return input[s : e-1], i, nil
}

// get array element(s) by index
//
// $[3] or $[-3] or $[1,2,-3]
// $..[3] or $..[1,2,-3]
//
// recurse inside
//
func arrayElemByIndex(input []byte, nod *tNode, inside bool) ([]byte, error) {
	var res []byte
	elems, elem, err := arrayIterateElems(input, nod)
	if err != nil {
		return nil, err
	}
	if len(nod.Elems) == 0 && nod.Slice[0] < 0 { // $[-3]
		i := len(elems) + nod.Slice[0]
		if i >= 0 && i < len(elems) {
			elem = input[elems[i].start:elems[i].end]
		}
	}
	if elem != nil { // $[3] or $[-3]
		res, err = getValue(elem, nod.Next, inside) // next node
		if err != nil || nod.Type&cDeep == 0 {
			return res, err
		}
	}
	// $[1,...] or $..[1,...]
	return collectRecurse(input, nod, elems, res, inside) // process elems + deepscan inside
}

// get array slice
//
// $[:3] or $[1:5:2] or $[:]
// $..[1:5] or $..[5:1:-1]
//
// recurse inside
//
func arraySlice(input []byte, nod *tNode) ([]byte, error) {
	elems, _, err := arrayIterateElems(input, nod)
	if err != nil {
		return nil, err
	}
	if len(elems) > 0 && nod.Type&cFullScan == 0 && nod.Next == nil {
		// 5.1)
		return input[elems[0].start:elems[len(elems)-1].end], nil
	}
	return sliceRecurse(input, nod, elems)
}

// iterate over array elements
//   cases
//     1) $[2]    cDot: nod.Left (>0)     --> seek to elem
//     2) $[2,3]  cDot: nod.Elems (>0)    --> scan collecting elems
//     3) $[-3]   cDot: nod.Left (<0)     --> full scan (cFullScan)
//     4) $[2,-3] cDot: nod.Elems (<0)    --> full scan (cFullScan)
//     5) $[1:3]  cSlice: Left < Right    --> scan up to right --> elems
//        5.1) terminal: return input[left:right]
//        5.2) non-term: iterate and recurse
//     6) .....   cSlice: other           --> full scan (cFullScan), apply bounds & recurse
//     7) cWild, cDeep:                   --> full scan (cFullScan), apply bounds & recurse
// returns
//   elem   - for a single index or cDeep
//   elems  - for a list of indexes or cDeep
//
func arrayIterateElems(input []byte, nod *tNode) (elems []tElem, elem []byte, err error) {
	var i, s, e int
	l := len(input)
	i = 1 // skip '['
BFOR:
	for pos := 0; i < l && input[i] != ']'; pos++ {
		s, e, i, err = valuate(input, i)
		if err != nil {
			return
		}
		found := nod.Type&(cFullScan|cWild|cDeep) > 0 // 3) 4) 6?) 7)
		for f := 0; !found && f < len(nod.Elems); f++ {
			if nod.Elems[f] == pos { // 2)
				found = true
			}
		}
		if nod.Slice[2] == 1 {
			switch nod.Type & (cDot | cSlice | cFullScan | cWild) {
			case cSlice: // 5)
				found = pos >= nod.Slice[0]
				if pos >= nod.Slice[1] {
					break BFOR
				}
			case cDot:
				if nod.Slice[0] == pos { // 1)
					elem = input[s:e]
					if nod.Type&cDeep == 0 {
						break BFOR // found single
					}
				}
			}
		}

		if found {
			elems = append(elems, tElem{s, e})
			if nod.Type&cFullScan == 0 && len(elems) == len(nod.Elems) {
				break // $[1,2,3] --> found them all
			}
		}
	}
	return
}

// aggregate non-empty elems and possibly non-empty ret
//
func collectRecurse(input []byte, nod *tNode, elems []tElem, res []byte, inside bool) ([]byte, error) {
	var err error

	if nod.Type&cFullScan == 0 || nod.Type&cWild > 0 {
		// special case 2): elems already listed
		for i := 0; i < len(elems); i++ {
			//if nod.Type&cWild > 0 {
			//	res = plus(res, input[elems[i].start:elems[i].end]) // wild
			//}
			res, err = subSlice(input, nod, elems, i, res, inside) // recurse + deep
			if err != nil {
				return res, err
			}
		}
		return res, err
	}
	// collect & recurse (cFullScan)
	for i := 0; i < len(nod.Elems); i++ {
		e := nod.Elems[i]
		if e < 0 {
			e += len(elems)
		}
		if e >= 0 && e < len(elems) {
			res, err = subSlice(input, nod, elems, e, res, inside) // recurse + deep
			if err != nil {
				return res, err
			}
		}
	}
	return res, err
}

// slice requested elements
//
// recurse on each element
//
// deepscan on each element if needed
func sliceRecurse(input []byte, nod *tNode, elems []tElem) ([]byte, error) {
	var res []byte
	var err error
	a, b, step, err := adjustBounds(nod.Slice[0], nod.Slice[1], nod.Slice[2], len(elems))
	if err != nil {
		return nil, err
	}
	if nod.Type&cWild > 0 {
		a, b, step = 0, len(elems), 1
	}
	if nod.Type&(cFullScan|cDeep|cWild) > 0 {
		for ; (a > b && step < 0) || (a < b && step > 0); a += step {
			res, err = subSlice(input, nod, elems, a, res, false) // TODO: make option to switch this to TRUE (nested aggregation)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// 5.2) special case: elems already filtered
		for i := 0; i < len(elems); i++ {
			res, err = subSlice(input, nod, elems, i, res, false) // TODO: make option to switch this to TRUE (nested aggregation)
			if err != nil {
				return nil, err
			}
		}
	}
	return res, err
}

func subSlice(input []byte, nod *tNode, elems []tElem, i int, res []byte, inside bool) ([]byte, error) {
	var sub []byte
	var err error
	if nod.Type&(cWild|cDeep) != cDeep {
		sub, err = getValue(input[elems[i].start:elems[i].end], nod.Next, inside)
		if err != nil {
			return nil, err
		}
		if len(sub) > 0 {
			res = plus(res, sub)
		}
	}
	if nod.Type&cDeep > 0 {
		sub, err = getValue(input[elems[i].start:elems[i].end], nod, true) // deepscan
		if len(sub) > 0 {
			res = plus(res, sub)
		}
	}
	return res, nil
}

/*
	input:
	 - key		= key that has been found
	 - input	= input[i]
	 - i		= points at value
	 - nod		= current node
	 - elems	=
	return:
	 - elems    = multikey or wild
	 - res	    = single key or deep
	 - i        =
	 - error    =
*/
func keyCheck(key []byte, input []byte, i int, nod *tNode, elems [][]byte, res []byte, inside bool) ([][]byte, []byte, int, error) {
	// var s, e int
	var err error
	// var val []byte

	// s, e, i, err = valuate(input, i) // s:e holds a value
	// if err != nil {
	// 	return elems, res, i, err
	// }
	// val = input[s:e]

	i, err = skipSpaces(input, i)
	if err != nil {
		return elems, res, i, err
	}

	b := i
	if nod.Type&cWild > 0 {
		elems, res, i, err = processKey(nod, nil, key, input, i, elems, res, false) // TODO: make option to switch the last FALSE to "inside" (nested aggregation)
	} else {
		for ii := range nod.Keys {
			elems, res, i, err = processKey(nod, nod.Keys[ii], key, input, i, elems, res, false) // TODO: make option to switch the last FALSE to "inside" (nested aggregation)
		}
	}

	if nod.Type&cDot > 0 && len(res) > 0 {
		return elems, res, i, err
	}

	if b == i {
		i, err = skipValue(input, i)
		if err != nil {
			return elems, res, i, err
		}
	}
	i, err = skipSpaces(input, i)
	return elems, res, i, err
}

func processKey(
	nod *tNode,
	nodkey []byte,
	key []byte,
	input []byte,
	i int,
	elems [][]byte,
	res []byte,
	inside bool,
) ([][]byte, []byte, int, error) {
	var err error
	var deep []byte
	var sub []byte
	e := i
	match := matchKeys(key, nodkey) || nod.Type&cWild > 0
	if nod.Type&cDeep > 0 || match {
		// key match
		if nod.Type&cDeep == 0 { // $.a  $.a.x  $[a,b]  $.*
			if len(nod.Keys) == 1 {
				// $.a  $.a.x
				res, err = getValue(input[i:], nod.Next, inside || nod.Type&cWild > 0) // recurse
				return elems, res, i, err
			}
			// $[a,b]  $[a,b].x  $.*
			e, err = skipValue(input, i)
			if err != nil {
				return elems, res, i, err
			}
			sub, err = getValue(input[i:e], nod.Next, inside || nod.Type&cWild > 0)
			if len(sub) > 0 {
				elems = append(elems, sub)
			}
		} else { // deep: $..a  $..[a,b]
			e, err = skipValue(input, i)
			if err != nil {
				return elems, res, i, err
			}
			if match {
				res = plus(res, input[i:e:e])
			}
			deep, err = getValue(input[i:e:e], nod, true) // deepscan
			if len(deep) > 0 {
				res = plus(res, deep)
			}
		}
	}
	return elems, res, e, err
}

func matchKeys(key []byte, nodkey []byte) bool {
	a, b := 0, 0
	la, lb := len(key), len(nodkey)
	for a < la && b < lb {
		ch := key[a]
		if ch == '\\' {
			a++
			if a == la {
				break
			}
			switch key[a] {
			case '"', '\\', '/':
				ch = key[a]
			case 'b', 'f', 'n', 'r', 't':
				ch = speChars[key[a]]
			case 'u':
				if a > la-5 || b > b-2 {
					break
				}
				bb := make([]byte, 2)
				_, err := hex.Decode(bb, key[a+1:a+5])
				if err != nil {
					return false
				}
				if bb[0] != nodkey[b] {
					return false
				}
				b++
				ch = bb[1]
			}
		}
		if ch != nodkey[b] {
			return false
		}
		a++
		b++
	}
	return a == la && b == lb
}

func valuate(input []byte, i int) (int, int, int, error) {
	var err error
	var s int
	s, err = skipSpaces(input, i)
	if err != nil {
		return 0, 0, i, err
	}
	e, err := skipValue(input, s) // s:e holds a value
	if err != nil {
		return s, e, i, err
	}
	i, err = skipSpaces(input, e)
	return s, e, i, err
}

func plus(res []byte, val []byte) []byte {
	if len(val) == 0 {
		return res
	}
	if len(res) > 0 {
		res = append(res[:len(res):len(res)], ',')
	}
	return append(res[:len(res):len(res)], val...)
}

type tElem struct {
	start int
	end   int
}

// always moving from start to end
// step sets the direction
// start bound always included
// non-empty end bound always excluded
//
// empty bound rules
//   positive step:
//     empty start = 0
//     empty end = last item (included)
//   negative step:
//     empty start = last item
//     empty end = first item (included)
//
func adjustBounds(start int, stop int, step int, n int) (int, int, int, error) {
	if n == 0 {
		return 0, 0, 0, nil
	}
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

func doFunc(input []byte, nod *tNode) ([]byte, error) {
	var err error
	var result int
	if bytes.Equal(word("size"), nod.Keys[0]) {
		result, err = skipValue(input, 0)
	} else if bytes.Equal(word("length"), nod.Keys[0]) || bytes.Equal(word("count"), nod.Keys[0]) {
		if input[0] == '"' {
			result, err = skipString(input, 0)
		} else if input[0] == '[' {
			i := 1
			l := len(input)
			// count elements
			for i < l && input[i] != ']' {
				_, _, i, err = valuate(input, i)
				if err != nil {
					return nil, err
				}
				result++
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
	i++
	for i < l && !(input[i] == unmark && nested == 0 && !instr) {
		ch := input[i]
		if ch == '\\' {
			i += 2
			if i >= l {
				return i, errUnexpectedEnd
			}
			continue
		}
		if ch == '"' {
			instr = !instr
		} else if !instr {
			if ch == mark {
				nested++
			} else if ch == unmark {
				nested--
			}
		}
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
