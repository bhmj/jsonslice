package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"
)

const (
	// array node
	cArrayType = 1 << iota
	// array properties
	cArrBounded = 1 << iota // bounded [x:y] or indexed [x]
	// terminal node
	cIsTerminal = 1 << iota
)

type tToken struct {
	Key   string
	Type  int8 // properties
	Left  int  // >=0 index from the start, <0 backward index from the end
	Right int  // 0 till the end inclusive, >0 to index exclusive, <0 backward index from the end exclusive
	Next  *tToken
}

// Get the jsonpath subset of the input
func Get(input []byte, path string) ([]byte, error) {

	if path[0] != '$' {
		return nil, errors.New("path: $ expected")
	}

	tokens, err := parsePath([]byte(path))
	if err != nil {
		return nil, err
	}

	return getValue(input, tokens)
}

func parsePath(path []byte) (*tToken, error) {
	tok := &tToken{}
	i := 0
	l := len(path)
	if l == 0 {
		return nil, errors.New("jsonpath: empty item")
	}
	// key
	for ; i < l && path[i] != '.' && path[i] != '['; i++ {
	}
	tok.Key = string(path[:i])
	// type
	if i == l {
		tok.Type |= cIsTerminal
		return tok, nil
	}
	if path[i] == '[' {
		tok.Type = cArrayType
		i++
		ind := 0
		ind, i = readNumber(path, i)
		if i == l || (path[i] != ':' && path[i] != ']') {
			return nil, errors.New("jsonpath: index bound missing")
		}
		tok.Left = ind
		//
		if path[i] == ':' {
			tok.Type |= cArrBounded
			i++
			ind, ii := readNumber(path, i)
			if (ind == 0 || ind == 1) && ii > i {
				return nil, errors.New("jsonpath: 0 or 1 as a second bound does not make sense")
			}
			if ii == l || path[ii] != ']' {
				return nil, errors.New("jsonpath: index bound missing")
			}
			i = ii
			tok.Right = ind
		}
		i++
		if i == l {
			tok.Type |= cIsTerminal
			return tok, nil
		}
	}
	if tok.Type&cArrBounded > 0 && tok.Type&cIsTerminal == 0 {
		return nil, errors.New("indefinite references are not yet supported")
	}
	if path[i] != '.' {
		return nil, errors.New("invalid element reference")
	}
	i++
	next, err := parsePath(path[i:])
	if err != nil {
		return nil, err
	}
	tok.Next = next

	return tok, nil
}

func getValue(input []byte, tok *tToken) (result []byte, err error) {
	// skip spaces
	i := 0
	l := len(input)
	for ch := input[i]; i < l && (ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'); ch = input[i] {
		i++
	}
	input = input[i:]
	if len(input) == 0 {
		return nil, errors.New("unexpected end of file")
	}
	if input[0] != '{' && input[0] != '[' {
		return nil, errors.New("object or array expected")
	}
	if tok.Key != "$" {
		// find the key and seek to the value
		input, err = getKeyValue(input, tok.Key)
		if err != nil {
			return nil, err
		}
	}
	// check value type
	if err = checkValueType(input, tok); err != nil {
		return nil, err
	}

	// here we are at the beginning of a value

	if tok.Type&cIsTerminal > 0 {
		if tok.Type&cArrayType > 0 {
			return sliceArray(input, tok)
		}
		eoe, err := skipValue(input, 0)
		if err != nil {
			return nil, err
		}
		return input[:eoe], nil
	}
	if tok.Type&cArrayType > 0 {
		input, err = sliceArray(input, tok)
	}
	return getValue(input, tok.Next)
}

const keySeek = 1
const keyOpen = 2
const keyClose = 4

// getKeyValue: find the key and seek to the value
func getKeyValue(input []byte, key string) ([]byte, error) {
	var err error
	if input[0] != '{' {
		return nil, errors.New("object expected")
	}

	i := 1
	l := len(input)

	for i < l && input[i] != '}' {
		state := keySeek
		k := make([]byte, 0)
		for ch := input[i]; i < l && state != keyClose; ch = input[i] {
			switch state {
			case keySeek:
				if ch == '"' {
					state = keyOpen
				}
			case keyOpen:
				if ch == '"' {
					state = keyClose
				} else {
					k = append(k, byte(ch))
				}
			}
			i++
		}

		if state == keyClose {
			i, err = seekToValue(input, i)
			if err != nil {
				return nil, err
			}
			if key == string(k) {
				return input[i:], nil
			}
			i, err = skipValue(input, i)
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, errors.New("field " + key + " not found")
}

// sliceArray select node(s) by bound(s)
func sliceArray(input []byte, tok *tToken) ([]byte, error) {
	var err error
	l := len(input)
	i := 1 // skip '['
	// backward index(es)
	elems := append([]int{}, i)
	// scan for elements
	for ch := input[i]; i < l && ch != ']'; ch = input[i] {
		i, err = skipValue(input, i)
		if err != nil {
			return nil, err
		}
		elems = append(elems, i)
	}
	//   select by index(es)
	if tok.Right == 0 {
		a := 0
		b := 0
		if tok.Left < 0 {
			a = len(elems) + tok.Left
			b = len(elems) + tok.Left + 1
		} else {
			a = tok.Left
			b = tok.Left + 1
		}
		if a < 0 || a >= len(elems) || b < 0 || b >= len(elems) {
			return nil, errors.New(tok.Key + "[" + strconv.Itoa(tok.Left) + "] does not exist")
		}
		return input[elems[a]:elems[b]], nil
	}
	// two bounds
	a := 0
	b := 0
	if tok.Left < 0 {
		a = len(elems) + tok.Left
	} else {
		a = tok.Left
	}
	if tok.Right < 0 {
		a = len(elems) + tok.Right
	} else {
		a = tok.Right
	}
	if a < 0 || a >= len(elems) || b < 0 || b >= len(elems) {
		return nil, errors.New(tok.Key + "[" + strconv.Itoa(tok.Left) + ":" + strconv.Itoa(tok.Right) + "] does not exist")
	}
	return input[a:b], nil
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
func getValues(input []byte, tok *tToken) ([]byte, error) {
	return nil, errors.New("not yet supported")
}

func seekToValue(input []byte, i int) (int, error) {
	l := len(input)
	// spaces
	for ch := input[i]; i < l && ch != ':'; ch = input[i] {
		i++
	}
	// colon
	if i == l {
		return 0, errors.New("unexpected end of input")
	}
	i++
	// spaces
	for ch := input[i]; i < l && (ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'); ch = input[i] {
		i++
	}
	if i == l {
		return 0, errors.New("unexpected end of input")
	}
	return i, nil
}

func skipValue(input []byte, i int) (int, error) {
	l := len(input)
	if i == l {
		return 0, errors.New("unexpected end of input")
	}
	// spaces
	for ch := input[i]; i < l && (ch == ',' || ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n'); ch = input[i] {
		i++
	}
	if i == l {
		return 0, errors.New("unexpected end of input")
	}
	if input[i] == '"' {
		// string
		prev := byte('"')
		done := false
		i++
		for ch := input[i]; i < l && !done; ch = input[i] {
			if ch == '"' && prev != '\\' {
				done = true
			}
			prev = ch
			i++
		}
		if i == l {
			return 0, errors.New("unexpected end of input")
		}
	} else if input[i] == '{' || input[i] == '[' {
		// object or array
		mark := input[i]
		unmark := mark + 2
		nested := 0
		instr := false
		prev := mark
		i++
		for ch := input[i]; i < l && !(ch == unmark && nested == 0); ch = input[i] {
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
		// number, bool, null
		for ch := input[i]; i < l && ch != ',' && ch != '}'; ch = input[i] {
			i++
		}
	}
	return i, nil
}

func checkValueType(input []byte, tok *tToken) error {
	if len(input) < 2 {
		return errors.New("unexpected end of input")
	}
	ch := input[0]
	if ch == '[' && tok.Type&cArrayType == 0 && tok.Type&cIsTerminal == 0 {
		return errors.New("object expected at " + tok.Key)
	} else if ch == '{' && tok.Type&cArrayType > 0 {
		return errors.New("array expected at " + tok.Key)
	} else if ch != '{' && ch != '[' && tok.Type&cIsTerminal == 0 {
		return errors.New("complex type expected at " + tok.Key)
	}
	return nil
}

// get input and current value position
// return next key and new value position

func nextKey(input []byte, i int) ([]byte, int) {
	status := keySeek
	key := make([]byte, 0)
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

// readNumber returns the array index specified in array bound clause
func readNumber(path []byte, i int) (int, int) {
	sign := 1
	l := len(path)
	ind := 0
	for ch := path[i]; i < l && (ch == '-' || (ch >= '0' && ch <= '9')); ch = path[i] {
		if ch == '-' {
			sign = -1
		} else {
			ind = ind*10 + int(ch-'0')
		}
		i++
	}
	return ind * sign, i
}

/*
	data := []byte(`
		{
			"store": {
				"book": [
					{
						"category": "reference",
						"author": "Nigel Rees",
						"title": "Sayings of the Century",
						"price": 8.95
					},
					{
						"category": "fiction",
						"author": "Evelyn Waugh",
						"title": "Sword of Honour",
						"price": 12.99
					},
					{
						"category": "fiction",
						"author": "Herman Melville",
						"title": "Moby Dick",
						"isbn": "0-553-21311-3",
						"price": 8.99
					},
					{
						"category": "fiction",
						"author": "J. R. R. Tolkien",
						"title": "The Lord of the Rings",
						"isbn": "0-395-19395-8",
						"price": 22.99
					}
				],
				"bicycle": {
					"color": "red",
					"price": 19.95
				}
			},
			"expensive": 10
		}
	`
*/

func main() {
	//	data := []byte(`{"store":{"book":[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99},{"category":"fiction","author":"Herman Melville","title":"Moby Dick","isbn":"0-553-21311-3","price":8.99},{"category":"fiction","author": "J. R. R. Tolkien","title": "The Lord of the Rings","isbn": "0-395-19395-8","price": 22.99}],"bicycle": {"color": "red","price": 19.95}},"expensive": 10}`)
	data := []byte(`{"store":{"book":[{"category":"reference","author":"Nigel Rees","title":"Sayings of the Century","price":8.95},{"category":"fiction","author":"Evelyn Waugh","title":"Sword of Honour","price":12.99}],"bicycle": {"color": "red","price": 19.95}},"expensive": 10}`)

	if len(os.Args) < 2 {
		fmt.Println("Usage: jsonslice <jsonpath>\n  ex: $.store.book[0].author")
		return
	}

	arg := os.Args[1]

	s, err := Get(data, arg)

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(s))
}
