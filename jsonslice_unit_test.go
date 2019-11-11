package jsonslice

import (
	"testing"
)

func Test_ajustBounds(t *testing.T) {
	type Input struct{ left, right, step int }
	type Expected struct {
		a, b, step int
		err        bool
	}
	n := 5
	tests := []struct {
		Input
		Expected
	}{
		// slice: [0,1,2,3,4]

		// [:] -> [0..5)
		{Input{cEmpty, cEmpty, cEmpty}, Expected{0, 5, 1, false}},
		// [2:] -> [2..5)
		{Input{2, cEmpty, cEmpty}, Expected{2, 5, 1, false}},
		// [:3] -> [0..3)
		{Input{cEmpty, 3, cEmpty}, Expected{0, 3, 1, false}},
		// [-2:] -> [3..5)
		{Input{-2, cEmpty, cEmpty}, Expected{3, 5, 1, false}},
		// [:-2] -> [0..3)
		{Input{cEmpty, -2, cEmpty}, Expected{0, 3, 1, false}},
		// [1:4] -> [1..4)
		{Input{1, 4, cEmpty}, Expected{1, 4, 1, false}},
		// [-4:4] -> [1..4)
		{Input{-4, 4, cEmpty}, Expected{1, 4, 1, false}},
		// [-8:4] -> [0..4)
		{Input{-8, 4, cEmpty}, Expected{0, 4, 1, false}},
		// [-8:8] -> [0..5)
		{Input{-8, 8, cEmpty}, Expected{0, 5, 1, false}},
		// [-8:8] -> [0..5)
		{Input{-8, 8, cEmpty}, Expected{0, 5, 1, false}},
		// [1:-2] -> [1..3)
		{Input{1, -2, cEmpty}, Expected{1, 3, 1, false}},
		// [1:-3] -> [1..2)
		{Input{1, -3, cEmpty}, Expected{1, 2, 1, false}},
		// [1:-4] -> [1..1)
		{Input{1, -4, cEmpty}, Expected{1, 1, 1, false}},
		// [-5:-2] -> [0..3)
		{Input{-5, -2, cEmpty}, Expected{0, 3, 1, false}},
	}

	for _, tst := range tests {
		a, b, step, err := adjustBounds(tst.Input.left, tst.Input.right, tst.Input.step, n)
		if !(a == tst.Expected.a && b == tst.Expected.b && step == tst.Expected.step) || (err != nil) != tst.Expected.err {
			t.Errorf(
				"adjustBounds(%v) == {%v,%v,%v,%v}, expected %v",
				tst.Input, a, b, step, err, tst.Expected,
			)
		}
	}
}

func Test_sliceRecurse(t *testing.T) {
	input := []byte(`["a","b","c","d","e"]`)
	elems := []tElem{{1, 4}, {5, 8}, {9, 12}, {13, 16}, {17, 20}}
	tests := []struct {
		nod      *tNode
		expected string
	}{
		// [:]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: cEmpty}, `"a","b","c","d","e"`},
		// [2:]
		{&tNode{Left: 2, Right: cEmpty, Step: cEmpty}, `"c","d","e"`},
		// [:3]
		{&tNode{Left: cEmpty, Right: 3, Step: cEmpty}, `"a","b","c"`},
		// [-2:]
		{&tNode{Left: -2, Right: cEmpty, Step: cEmpty}, `"d","e"`},
		// [:-2]
		{&tNode{Left: cEmpty, Right: -2, Step: cEmpty}, `"a","b","c"`},
		// [1:4]
		{&tNode{Left: 1, Right: 4, Step: cEmpty}, `"b","c","d"`},
		// [-4:4]
		{&tNode{Left: -4, Right: 4, Step: cEmpty}, `"b","c","d"`},
		// [-8:4]
		{&tNode{Left: -8, Right: 4, Step: cEmpty}, `"a","b","c","d"`},
		// [-8:8]
		{&tNode{Left: -8, Right: 8, Step: cEmpty}, `"a","b","c","d","e"`},
		// [1:-2]
		{&tNode{Left: 1, Right: -2, Step: cEmpty}, `"b","c"`},
		// [1:-3]
		{&tNode{Left: 1, Right: -3, Step: cEmpty}, `"b"`},
		// [1:-4]
		{&tNode{Left: 1, Right: -4, Step: cEmpty}, ``},
		// [-5:-2] -> 0..2
		{&tNode{Left: -5, Right: -2, Step: cEmpty}, `"a","b","c"`},

		// slice + step
		// [::1]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: 1}, `"a","b","c","d","e"`},
		// [1::2]
		{&tNode{Left: 1, Right: cEmpty, Step: 2}, `"b","d"`},

		// slice + step
		// [::-1]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: -1}, `"e","d","c","b","a"`},
		// [2::-1]
		{&tNode{Left: 2, Right: cEmpty, Step: -1}, `"c","b","a"`},
		// [:3:-1]
		{&tNode{Left: cEmpty, Right: 3, Step: -1}, `"e"`},
		// [-2::-1]
		{&tNode{Left: -2, Right: cEmpty, Step: -1}, `"d","c","b","a"`},
		// [-2:1:-1]
		{&tNode{Left: -2, Right: 1, Step: -1}, `"d","c"`},
		// [::-2]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: -2}, `"e","c","a"`},
		// [::-3]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: -3}, `"e","b"`},
		// [::-4]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: -4}, `"e","a"`},
		// [::-5]
		{&tNode{Left: cEmpty, Right: cEmpty, Step: -5}, `"e"`},
		// [1:2:-1]
		{&tNode{Left: 1, Right: 2, Step: -1}, ``},
	}

	for _, tst := range tests {
		res, err := sliceRecurse(input, tst.nod, elems)
		if err != nil || tst.expected != string(res) {
			t.Errorf(
				"sliceRecurse('%v', {%d, %d, %d}) == %v, expected %v",
				string(input), tst.nod.Left, tst.nod.Right, tst.nod.Step, string(res), tst.expected,
			)
		}
	}
}
