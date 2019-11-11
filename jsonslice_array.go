package jsonslice

/**
  JsonSlice 0.7.4
  Michael Gurov, 2018-2019
  MIT licenced

  Slice a part of a raw json ([]byte) using jsonpath, without unmarshalling the whole thing.
  The result is also []byte.
**/

func init() {
}

// GetArrayElements returns a slice of array elements (in raw, i.e. []byte), matching jsonpath.
// Note that an array reference must be the only array and the last one in path, for example:
// "$[:-1]" is ok, "$.foo.bar[:]" is ok, "$.foo[:].bar" is not, "foo[:].bar[:]" is not
func GetArrayElements(input []byte, path string, alloc int) ([][]byte, error) {

	if len(path) == 0 {
		return nil, errPathEmpty
	}

	if path[0] != '$' {
		return nil, errPathRootExpected
	}

	node, _, err := parsePath([]byte(path))
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

	return getValueAE(input, node, alloc)
}

func getValueAE(input []byte, nod *tNode, alloc int) (result [][]byte, err error) {

	i, _ := skipSpaces(input, 0)

	input = input[i:]
	if err = looksLikeJSON(input); err != nil {
		return nil, err
	}
	// wildcard
	if len(nod.Key) == 1 && nod.Key[0] == '*' {
		return nil, errWildcardsNotSupported
	}
	if len(nod.Keys) > 0 || (len(nod.Key) > 0 && !bytein(nod.Key[0], []byte{'$', '@'})) {
		// find the key and seek to the value
		if input, err = getKeyValue(input, nod); err != nil {
			return nil, err
		}
	}
	// check value type
	if err = checkValueType(input, nod); err != nil {
		return nil, err
	}

	// here we are at the beginning of a value

	if nod.Type&cSubject > 0 {
		return nil, errFunctionsNotSupported
	}
	if nod.Type&cIsTerminal > 0 && input[i] != '[' {
		return nil, errTerminalNodeArray
	}
	return sliceArrayElements(input, nod, alloc)

	/*
		if nod.Type&cArrayType > 0 {
			if nod.Type&cAgg > 0 {
				return nil, errSubslicingNotSupported
			}
			if input, err = sliceArray(input, nod); err != nil {
				return nil, err
			}
		}
	*/
	//return getValueAE(input, nod.Next, alloc)
}

// sliceArrayElements returns a slice of array elements
func sliceArrayElements(input []byte, nod *tNode, alloc int) ([][]byte, error) {
	if input[0] != '[' {
		return nil, errArrayExpected
	}
	i := 1 // skip '['

	res := make([][]byte, 0, alloc)

	if /*nod.Type&cArrayRanged == 0 &&*/ nod.Left >= 0 && len(nod.Elems) == 0 {
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
	if nod.Type&cSlice == 0 {
		a := nod.Left + len(elems) // nod.Left is negative, so correct it to a real element index
		if a < 0 {
			return nil, errArrayElementNotFound
		}

		return append(res, input[elems[a].start:elems[a].end]), nil
	}
	// two bounds
	a, b, _, err := adjustBounds(nod.Left, nod.Right, 1, len(elems))
	if err != nil {
		return nil, err
	}
	for ; a <= b; a++ {
		res = append(res, input[elems[a].start:elems[a].end])
	}
	return res, nil
}
