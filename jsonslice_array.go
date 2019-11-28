package jsonslice

import (
	"errors"
	"strconv"
)

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

	node, i, err := readRef([]byte(path), 1, 0)
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
				if tok.Operand != nil && tok.Operand.Node != nil && tok.Operand.Type&cRoot > 0 {
					val, err := getValue2(input, tok.Operand.Node, false)
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

	//result, err := getValueAE(input, node, alloc, false)
	repool(node)
	return nil, err
}
