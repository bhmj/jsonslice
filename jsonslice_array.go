package jsonslice

/**
  JsonSlice 0.7.3
  Michael Gurov, 2018-2019
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

import (
	"errors"
)

func init() {
}

// GetArrayElements returns a slice of array elements (in raw, i.e. []byte), matching jsonpath.
// Note that an array reference must be the only array and the last one in path, for example:
// "$[:-1]" is ok, "$.foo.bar[:]" is ok, "$.foo[:].bar" is not, "foo[:].bar[:]" is not
func GetArrayElements(input []byte, path string, alloc int) ([][]byte, error) {

	if len(path) == 0 {
		return nil, errors.New("path: empty")
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
						tok.Operand.Type = cOpNull
					}
					decodeValue(val, tok.Operand)
					tok.Operand.Node = nil
				}
			}
		}
	}

	return getValueAE(input, node, alloc)
}

func getValueAE(input []byte, nod *tNode, alloc int) (result [][]byte, err error) {

	i, _ := skipSpaces(input, 0)

	input = input[i:]
	if len(input) == 0 {
		return nil, errors.New("unexpected end of input")
	}
	if !bytein(input[0], []byte{'{', '['}) {
		return nil, errors.New("object or array expected")
	}
	// wildcard
	if nod.Key == "*" {
		return nil, errors.New("wildcards are not supported in GetArrayElements")
	}
	if nod.Key != "$" && nod.Key != "@" && (nod.Key != "" || len(nod.Keys) > 0) {
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
		return nil, errors.New("functions are not supported in GetArrayElements")
	}
	if nod.Type&cIsTerminal > 0 {
		if nod.Type&cArrayType == 0 {
			return nil, errors.New("functions are not supported in GetArrayElements")
		}
		return sliceArrayElements(input, nod, alloc)
	}
	if nod.Type&cArrayType > 0 {
		if input, err = sliceArray(input, nod); err != nil {
			return nil, err
		}
		if nod.Type&cAgg > 0 {
			return nil, errors.New("sub-slicing is not supported in GetArrayElements")
		}
	}
	return getValueAE(input, nod.Next, alloc)
}

// sliceArrayElements returns a slice of array elements
func sliceArrayElements(input []byte, nod *tNode, alloc int) ([][]byte, error) {
	if input[0] != '[' {
		return nil, errors.New("array expected at " + nod.Key)
	}
	i := 1 // skip '['

	res := make([][]byte, 0, alloc)

	if nod.Type&cArrayRanged == 0 && nod.Left >= 0 && len(nod.Elems) == 0 {
		// single positive index -- easiest case
		elem, err := getArrayElement(input, i, nod)
		if err != nil {
			return nil, err
		}
		return append(res, elem), nil
	}
	// if nod.Filter != nil {
	// 	// filtered array
	// 	return getFilteredElements(input, i, nod)
	// }

	// fullscan
	var elems []tElem
	var err error
	elems, err = arrayScan(input)
	if err != nil {
		return nil, err
	}
	if len(nod.Elems) > 0 {
		for _, ii := range nod.Elems {
			res = append(res, input[elems[ii].start:elems[ii].end])
		}
		return res, nil
	}
	//   select by index(es)
	if nod.Type&cArrayRanged == 0 {
		a := nod.Left + len(elems) // nod.Left is negative, so correct it to a real element index
		if a < 0 {
			return nil, errors.New("specified element not found")
		}

		return append(res, input[elems[a].start:elems[a].end]), nil
	}
	// two bounds
	a, b, err := adjustBounds(nod.Left, nod.Right, len(elems))
	if err != nil {
		return nil, err
	}
	for ; a <= b; a++ {
		res = append(res, input[elems[a].start:elems[a].end])
	}
	return res, nil
}
