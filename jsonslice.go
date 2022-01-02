package jsonslice

/**
  JsonSlice 1.1.0
  By Michael Gurov, 2018-2021
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

import (
	"bytes"
	"errors"
	"strconv"
	"sync"

	"github.com/bhmj/xpression"
)

var (
	nodePool sync.Pool

	errPathEmpty,
	errPathInvalidChar,
	errPathRootExpected,
	errPathUnexpectedEnd,
	errPathUnknownEscape,
	errPathUnknownFunction,
	errFieldNotFound,
	errColonExpected,
	errUnrecognizedValue,
	errUnexpectedEnd,
	errInvalidLengthUsage,
	errUnexpectedStringEnd,
	errObjectOrArrayExpected error
)

func init() {
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
	errPathUnknownEscape = errors.New("path: unknown escape")
	errPathUnknownFunction = errors.New("path: unknown function")
	errFieldNotFound = errors.New(`field not found`)
	errColonExpected = errors.New("':' expected")
	errUnrecognizedValue = errors.New("unrecognized value: true, false or null expected")
	errUnexpectedEnd = errors.New("unexpected end of input")
	errInvalidLengthUsage = errors.New("length() is only applicable to array or string")
	errObjectOrArrayExpected = errors.New("object or array expected")
	errUnexpectedStringEnd = errors.New("unexpected end of string")
}

type word []byte

const (
	cDot      = 1 << iota // 1 common [dot-]node
	cAgg      = 1 << iota // 2 aggregating
	cFunction = 1 << iota // 4 function
	cSlice    = 1 << iota // 8 slice array [x:y:s]
	cFullScan = 1 << iota // 16 array slice: need fullscan
	cFilter   = 1 << iota // 32 filter
	cWild     = 1 << iota // 64 wildcard (*)
	cDeep     = 1 << iota // 128 deepscan (..)

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
	Filter []*xpression.Token
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
//   1) simple case: the result is a simple subslice of a source input.
//   2) the result is a merge of several non-contiguous parts of input. More allocations are needed.
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
			for i, tok := range n.Filter {
				if tok.Type == xpression.VariableOperand && tok.Operand.Str[0] == '$' {
					// every variable has an empty token right after it for storing the result
					result := n.Filter[i+1]
					// evaluate root-based reference
					val, err := Get(input, string(tok.Operand.Str))
					if err != nil {
						// not found or other error
						result.Type = xpression.UndefinedOperand
					}
					_ = decodeValue(val, &result.Operand)
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

var pathTerminator = []byte{' ', '\t', '<', '=', '>', '+', '-', '*', '/', ')', '&', '|', '!', '^'}

var keyTerminator = []byte{' ', '\t', ':', '.', ',', '[', '(', ')', ']', '<', '=', '>', '+', '-', '*', '/', '&', '|', '!'}

// readRef recursively reads input path until EOL or path terminator encountered.
// Returns single-linked list of nodes, end position or error.
func readRef(path []byte, i int, uptype int) (*tNode, int, error) {
	var err error
	var next *tNode
	var sep byte
	var flags int
	var key word

	if i >= len(path) {
		// EOL encountered
		return nil, i, nil
	}

	if bytein(path[i], pathTerminator) {
		// path terminator encountered
		return nil, i, nil
	}

	if !bytein(path[i], []byte{'.', '['}) {
		// only dot and bracket notation allowed
		return nil, i, errPathInvalidChar
	}

	nod := getEmptyNode()
	l := len(path)
	// [optional] dots
	if path[i] == '.' {
		nod.Type = cDot // simple dor notation
		if i+1 < l && path[i+1] == '.' {
			nod.Type = cDeep // .. means deepscan
			i++
		}
		i++
		if i == l {
			return nil, i, errPathUnexpectedEnd // need key after dot(s)
		}
	}

	// NOTE: this sequence of blocks supports .[] notation. Maybe should restrict that?

	if path[i] == '[' {
		// bracket notated
		i++
		i, err = readBrackets(nod, path, i)
		if i == l || err != nil {
			return nod, i, err
		}
	} else {
		// dot (or deepscan) notated
		key, nod.Slice[0], sep, i, flags, _ = readKey(path, i)
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
	}

	// recurse
	next, i, err = readRef(path, i, nod.Type)
	nod.Next = next
	return nod, i, err
}

// readBrackets read bracket-notated expression.
//
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
		// ?(...): filter
		return readFilter(path, i+2, nod)
	}
	for pos := 0; i < l && path[i] != ']'; pos++ {
		key, ikey, sep, i, flags, err = readKey(path, i)
		nod.Type |= flags // cWild, cFullScan // CAUTION: [*,1,2] is possible
		if err != nil {
			return i, err
		}
		err = setupNode(nod, key, ikey, sep, pos)
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

// readKey reads next key from path[i].
// Key must be any of the following: quoted string, word bounded by keyTerminator, *, 123
// returns:
//   key   = the key
//   ikey  = integer converted key
//   sep   = key list separator (expected , : [ ] . +-*/=! 0)
//   i     = current i (on separator)
//   flags = cWild if wildcard
//   err   = error
func readKey(path []byte, i int) ([]byte, int, byte, int, int, error) {
	l := len(path)
	var bound byte
	var key []byte
	var err error
	var flag int

	if i == l {
		return nil, 0, 0, i, 0, errPathUnexpectedEnd
	}

	if bytein(path[i], []byte{'\'', '"'}) {
		// quoted string
		key, i, err = readQuotedKey(path, i)
	} else {
		// terminator bounded string
		key, i, err = readTerminatorBounded(path, i, keyTerminator)
		if len(key) == 1 && key[0] == '*' {
			flag = cWild
			key = key[:0]
		}
	}
	if err != nil {
		return nil, 0, 0, i, 0, err
	}

	if i < l {
		bound = path[i]
	} else {
		bound = 0
	}
	ikey := toInt(key)
	if ikey < 0 || ikey == cEmpty {
		flag |= cFullScan // fullscan if $[-1], $[1,-1] or $[1:-1] or $[1:]
	}
	return key, ikey, bound, i, flag, nil
}

// setupNode sets up node Type and either (appens Keys or Elems) or (fills up Slice) depending on note Type
func setupNode(nod *tNode, key []byte, ikey int, sep byte, pos int) error {
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

// returns value specified by nod or nil if no match
// 'inside' specifies recursive mode
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
		b, err = filterMatch(input[s:e], nod.Filter)
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
	key, i, err := readQuotedKey(input, i)
	if err != nil {
		return nil, i, err
	}
	i, err = seekToValue(input, i)
	if err != nil {
		return nil, i, err
	}
	return key, i, nil
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
		sub, _ = getValue(input[elems[i].start:elems[i].end], nod, true) // deepscan
		if len(sub) > 0 {
			res = plus(res, sub)
		}
	}
	return res, nil
}

// Check for key match (or wildscan)
// "key" has been found earlier in input json
// If match then get value, if not match then skip value
// return "res" with a value and "i" pointing after the value
//
func keyCheck(key []byte, input []byte, i int, nod *tNode, elems [][]byte, res []byte, inside bool) ([][]byte, []byte, int, error) {
	var err error

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
			/*
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
			*/
			e, err = skipValue(input, i)
			if err != nil {
				return elems, res, i, err
			}
			if match {
				sub, _ = getValue(input[i:e:e], nod.Next, inside || nod.Type&cWild > 0)
				if len(sub) > 0 {
					res = plus(res, sub)
				}
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
		if ch != nodkey[b] {
			return false
		}
		a++
		b++
	}
	return a == la && b == lb
}

// get current value from input
// returns
//   s = start of value
//   e = end of value
//   i = next item
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

// always moving from start to end;
// step sets the direction;
// start bound always included;
// non-empty end bound always excluded;
//
// empty bound rules:
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
