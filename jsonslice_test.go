package jsonslice

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/oliveagle/jsonpath"
)

var data []byte
var condensed []byte

func init() {
	data = []byte(`
		{
			"store": {
				"open": true,
				"branch": null,
				"manager": [],
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
					"price": 19.95,
					"equipment": [
						["paddles", "umbrella", "horn"],
						["peg leg", "parrot", "map"],
						["light saber", "apparel"],
						["\"quoted\""]
					]
				}
			},
			"expensive": 10
		}
	`)
	condensed = []byte(`
		{
			"store": {
				"open": true,
				"branch": null,
				"manager": [],
				"book": [
					{"category":"reference", "author":"Nigel Rees", "title":"Sayings of the Century", "price":8.95},
					{"category":"fiction", "author":"Evelyn Waugh", "title":"Sword of Honour", "price":12.99},
					{"category":"fiction", "author":"Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99},
					{"category":"fiction", "author":"J. R. R. Tolkien",	"title":"The Lord of the Rings", "isbn":"0-395-19395-8", "price":22.99}
				],
				"bicycle": {
					"color": "red",
					"price": 19.95,
					"equipment": [
						["paddles", "umbrella", "horn"],
						["peg leg", "parrot", "map"],
						["light saber", "apparel"],
						["\"quoted\""]
					]
				}
			},
			"expensive": 10
		}
	`)
}

func randomBytes(p []byte, min, max int) {
	for i := 0; i < len(p); i++ {
		p[i] = byte(rand.Intn(max-min) + min)
	}
}

func TestFuzzyPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	var str string
	defer func() {
		if v := recover(); v != nil {
			println("'" + hex.EncodeToString([]byte(str)) + "'")
			println("'" + str + "'")
			panic(v)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 100)
	top := 2000000
	fmt.Printf("\rpath fuzzy [                    ]\rpath fuzzy [")
	for i := 0; i < top; i++ {
		if i%(top/20) == 1 {
			fmt.Printf(".")
		}
		randomBytes(b, 32, 127)
		n := rand.Intn(len(b)) + 1
		b[0] = '$'
		str = string(b[:n])
		readRef([]byte(str), 1, 0)
	}
	fmt.Println()
}

func TestCustomPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	readRef([]byte("$.()"), 1, 0)
}

func TestFuzzyGet(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	var str string
	defer func() {
		if v := recover(); v != nil {
			println("'" + hex.EncodeToString([]byte(str)) + "'")
			println("'" + str + "'")
			panic(v)
		}
	}()
	rand.Seed(time.Now().UnixNano())
	b := make([]byte, 500)
	top := 10000000
	fmt.Printf("\rget fuzzy  [                    ]\rget fuzzy  [")
	for i := 0; i < top; i++ {
		if i%(top/20) == 1 {
			fmt.Printf(".")
		}
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		str = string(b[:n])
		Get([]byte(str), "$.some.value")
	}
	fmt.Println()
}

func Test_10Mb(t *testing.T) {
	largeData := GenerateLargeData()
	expected := []byte(`"Sword of Honour"`)
	path := "$.store.book[100000].title"
	res, err := Get(largeData, path)
	if compareSlices(res, expected) != 0 && err == nil {
		t.Errorf(path + "\nexpected:\n" + string(expected) + "\ngot:\n" + string(res))
	}
}

func Test_Expressions(t *testing.T) {

	tests := []struct {
		Query    string
		Expected []byte
	}{
		// self
		{`$`, data},
		// simple query
		{`$.expensive`, []byte(`10`)},
		// simple query
		{`$.store.book[3].author`, []byte(`"J. R. R. Tolkien"`)},

		// boolean, null
		{`$.store.open`, []byte(`true`)},
		{`$.store.branch`, []byte(`null`)},

		// negative index
		{`$.store.book[-1].author`, []byte(`"J. R. R. Tolkien"`)},
		// negative indexes
		{`$.store.book[-3:-2].author`, []byte(`["Evelyn Waugh"]`)},

		// functions
		{`$.store.book.length()`, []byte(`4`)},
		{`$.store.book.count()`, []byte(`4`)},
		{`$.store.book.size()`, []byte(`604`)},

		// aggregated
		{`$.store.book[1:3].author`, []byte(`["Evelyn Waugh","Herman Melville"]`)},
		// aggregated, skip missing keys
		{`$.store.book[1:].isbn`, []byte(`["0-553-21311-3","0-395-19395-8"]`)},
		// aggregated, enumerate indexes
		{`$.store.book[0,2].title`, []byte(`["Sayings of the Century","Moby Dick"]`)},

		// filters: simple expression
		{`$.store.book[?(@.price>10)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
		// filters: simple expression + spaces
		{`$.store.book[?( @.price < 10 )].title`, []byte(`["Sayings of the Century","Moby Dick"]`)},
		// filters: simple expression
		{`$.store.book[?(@.price==12.99)].title`, []byte(`["Sword of Honour"]`)},
		// more spaces
		{`$.store.book[?(   @.price   >   10  )].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},

		// field presence
		{`$.store.book[?(@.isbn)].title`, []byte(`["Moby Dick","The Lord of the Rings"]`)},
		// string match
		{`$.store.book[?(@.isbn == "0-553-21311-3")].title`, []byte(`["Moby Dick"]`)},
		// string mismatch
		{`$.store.book[?(@.isbn != "0-553-21311-3")].title`, []byte(`["The Lord of the Rings"]`)},
		// root references
		{`$.store.book[?(@.price > $.expensive)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
		// math +
		{`$.store.book[?(@.price > $.expensive+1)].price`, []byte(`[12.99,22.99]`)},
		// math -
		{`$.store.book[?(@.price > $.expensive-1)].price`, []byte(`[12.99,22.99]`)},
		// math *
		{`$.store.book[?(@.price > $.expensive*1.1)].price`, []byte(`[12.99,22.99]`)},
		// math /
		{`$.store.book[?(@.price > $.expensive/0.7)].price`, []byte(`[22.99]`)},
		// logic operators : AND
		{`$.store.book[?(@.price > $.expensive && @.isbn)].title`, []byte(`["The Lord of the Rings"]`)},
		// logic operators : OR
		{`$.store.book[?(@.price >= $.expensive || @.isbn)].title`, []byte(`["Moby Dick","The Lord of the Rings"]`)},
		// logic operators : AND/OR numbers, strings
		{`$.store.book[?(@.price || @.isbn != "")].title`, []byte(`["Sayings of the Century","Sword of Honour","Moby Dick","The Lord of the Rings"]`)},
		// logic operators : same as above, for coverage's sake
		{`$.store.book[?(@.isbn != "" || @.price)].title`, []byte(`["Sayings of the Century","Sword of Honour","Moby Dick","The Lord of the Rings"]`)},
		// non-empty field
		{`$.store.book[?(@.price)].price`, []byte(`[8.95,12.99,8.99,22.99]`)},
		// bool ==
		{`$.store.book[?($.store.open == true)].price`, []byte(`[8.95,12.99,8.99,22.99]`)},
		// bool !=
		{`$.store.book[?($.store.open != false)].price`, []byte(`[8.95,12.99,8.99,22.99]`)},
		// bool <
		{`$.store.book[?($.store.open > false)].price`, []byte(`[8.95,12.99,8.99,22.99]`)},
		// bool >
		{`$.store.book[?($.store.open < true)].price`, []byte(`[]`)},
		// escaped chars
		{`$.store.bicycle.equipment[?(@[0] == "\"quoted\"")]`, []byte(`[["\"quoted\""]]`)},
		// numbers
		{`$.store.book[?(20 <= @.price)].title`, []byte(`["The Lord of the Rings"]`)},
		// numbers (+)
		{`$.store.book[?(@.price != 22.99)].price`, []byte(`[8.95,12.99,8.99]`)},

		// regexp
		{`$.store.book[?(@.title =~ /the/i)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},
		// regexp
		{`$.store.book[?(@.title =~ /(Saying)|(Lord)/)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},

		// array of arrays
		{`$.store.bicycle.equipment[1][0]`, []byte(`"peg leg"`)},
		// filter expression not found -- not an error
		//{`$.store.book[?($.store[0] > 0)]`, []byte(`[]`)},

		// wildcard: any key within an object
		{`$.store.book[0].*`, []byte(`["reference","Nigel Rees","Sayings of the Century",8.95]`)},
		// wildcard: named key on a given level
		{`$.store.*.price`, []byte(`[19.95]`)},

		// wildcard: named key in any array on a given level
		{`$.store.*[:].price`, []byte(`[8.95,12.99,8.99,22.99]`)},

		// all elements of an empty array is an empty array
		{`$.store.manager[:]`, []byte(`[]`)},
		// multiple keys (output ordered as in data)
		{`$.store.book[:]['price','title']`, []byte(`["Sayings of the Century",8.95,"Sword of Honour",12.99,"Moby Dick",8.99,"The Lord of the Rings",22.99]`)},
		// multiple keys combined with filter
		{`$.store.book[?(@.price > $.expensive*1.1)]['price','title']`, []byte(`["Sword of Honour",12.99,"The Lord of the Rings",22.99]`)},
		// functions in filter
		{`$.store.bicycle.equipment[?(@.count() == 2)][1]`, []byte(`["apparel"]`)},
	}

	for _, tst := range tests {
		res, err := Get(data, tst.Query)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if compareSlices(res, tst.Expected) != 0 {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(res) + "`")
		}
	}
}

func Test_Extensions(t *testing.T) {
	variant1 := []byte(`
							{
								"book": [
									{"Author": "J.R.R.Tolkien", "Title": "Lord of the Rings"},
									{"Author": "Z.Hopp", "Title": "Trollkrittet"}
								]
							}
						`)
	variant2 := []byte(`{ "book": [ {"Book one"}, {"Book two"}, {"Book three"}, {"Book four"} ] }`)
	variant3 := []byte(`{"a": "first", "2": "second", "b": "third"}`)
	variant4 := []byte(`["first", "second", "third"]`)
	variant5 := []byte(`["first", "second", "third", "fourth", "fifth"]`)
	variant6 := []byte(`{"key":"value"}`)
	variant7 := []byte(`{"key":"value", "another":"entry"}`)
	variant8 := []byte(`{"0":"value"}`)
	variant9 := []byte(`{"single'quote":"value"}`)
	variantA := []byte(`{"special:\"chars":"value"}`)
	variantB := []byte(`{"*":"value"}`)
	variantC := []byte(`["first",{"key":["first nested",{"more":[{"nested":["deepest","second"]},["more","values"]]}]}]`)
	variantD := []byte(`
							{
								"object": {
									"key": "value",
									"array": [
										{"key": "something"},
										{"key": {"key": "russian dolls"}}
									]
								},
								"key": "top"
							}
				`)
	variantE := []byte(`{"key":"value","another key": {"complex":"string","primitives":[0,1]}}`)
	variantF := []byte(`[40, null, 42]`)
	variantG := []byte(`42`)
	variantH := []byte(`["string", 42, {"key":"value"}, [0,1]]`)
	variantI := []byte(`{"some": "string", "int": 42, "object": {"key":"value"}, "array": [0,1]}`)
	variantJ := []byte(`[{"bar": [{"baz": "hello"}]}]`)

	tests := []struct {
		Query    string
		Base     []byte
		Expected []byte
	}{
		// custom extensions
		{`$.'book'[1]`, variant1, []byte(`{"Author": "Z.Hopp", "Title": "Trollkrittet"}`)},
		{`$.'book'.1`, variant1, []byte(`{"Author": "Z.Hopp", "Title": "Trollkrittet"}`)},
		// wildcard ignored if not alone (but still aggregates!)
		//{`$.book[1,*]`, variant1, []byte(`[{"Author": "Z.Hopp", "Title": "Trollkrittet"}]`)},

		// gold standard

		// array index dot notation
		{`$.book.2`, variant2, []byte(`{"Book three"}`)},
		// array index dot notation on object
		{`$.2`, variant3, []byte(`"second"`)},
		// array index slice end out of bounds
		{`$[1:10]`, variant4, []byte(`["second","third"]`)},
		// array index slice negative step
		{`$[::-2]`, variant5, []byte(`["fifth","third","first"]`)},
		// Array index slice start end negative step
		{`$[3:0:-2]`, variant5, []byte(`["fourth","second"]`)},
		// Array index slice start end step
		{`$[0:3:2]`, variant5, []byte(`["first","third"]`)},
		// Array index slice start end step 0
		{`$[0:3:0]`, variant5, []byte(`["first","second","third"]`)},
		// Array index slice start end step 1
		{`$[0:3:1]`, variant5, []byte(`["first","second","third"]`)},
		// Array index slice start end step non aligned
		{`$[0:4:2]`, variant5, []byte(`["first","third"]`)},
		// Array index slice start equals end
		{`$[0:0]`, variant5, []byte(`[]`)},
		// Array index slice step
		{`$[::2]`, variant5, []byte(`["first","third","fifth"]`)},

		// Key bracket notation
		{`$['key']`, variant6, []byte(`"value"`)},
		// Key bracket notation union
		{`$['key','another']`, variant7, []byte(`["value","entry"]`)},
		// Key bracket notation with double quotes
		{`$["key"]`, variant6, []byte(`"value"`)},
		// Key bracket notation with number
		{`$['0']`, variant8, []byte(`"value"`)},
		// Key bracket notation with number without quotes
		{`$[0]`, variant8, []byte(`"value"`)},
		// Key bracket notation with single quote

		// {`$['single'quote']`, variant9, []byte(``)}, -- this one MUST generate error
		// Key bracket notation with single quote escaped
		{`$['single\'quote']`, variant9, []byte(`"value"`)},
		// Key bracket notation with special characters
		{`$['special:"chars']`, variantA, []byte(`"value"`)},
		// Key bracket notation with star literal
		{`$['*']`, variantB, []byte(`"value"`)},
		// Key bracket notation without quotes
		{`$[key]`, variant6, []byte(`"value"`)},
		// Key dot bracket notation
		{`$.['key']`, variant6, []byte(`"value"`)},
		// Key dot bracket notation with double quotes
		{`$.["key"]`, variant6, []byte(`"value"`)},
		// Key dot bracket notation without quotes
		{`$.[key]`, variant6, []byte(`"value"`)},

		// Key dot notation with double quotes
		{`$."key"`, variant6, []byte(`"value"`)},
		// Key dot notation with single quotes
		{`$.'key'`, variant6, []byte(`"value"`)},
		// Recursive array index
		{`$..[0]`, variantC, []byte(`["first","first nested",{"nested":["deepest","second"]},"deepest","more"]`)},
		// Recursive key
		{`$..key`, variantD, []byte(`["value","something",{"key": "russian dolls"},"russian dolls","top"]`)},
		// Recursive key with double quotes
		{`$.."key"`, variantD, []byte(`["value","something",{"key": "russian dolls"},"russian dolls","top"]`)},
		// Recursive key with single quotes
		{`$..'key'`, variantD, []byte(`["value","something",{"key": "russian dolls"},"russian dolls","top"]`)},
		// Recursive on nested object
		{`$.store..price`, data, []byte(`[8.95,12.99,8.99,22.99,19.95]`)},

		// Recursive wildcard
		{`$..*`, variantE, []byte(`["value",{"complex":"string","primitives":[0,1]},"string",[0,1],0,1]`)},
		// Recursive wildcard on null value array
		{`$..*`, variantF, []byte(`[40,null,42]`)},
		// Recursive wildcard on scalar
		{`$..*`, variantG, []byte(`[]`)},

		// Wildcard bracket notation on array
		{`$[*]`, variantH, []byte(`["string",42,{"key":"value"},[0,1]]`)},
		// Wildcard bracket notation on null value array
		{`$[*]`, variantF, []byte(`[40,null,42]`)},
		// Wildcard bracket notation on object
		{`$[*]`, variantI, []byte(`["string",42,{"key":"value"},[0,1]]`)},
		// Wildcard bracket notation with key on nested objects
		{`$[*].bar[*].baz`, variantJ, []byte(`["hello"]`)},
		// Wildcard dot notation on array
		{`$.*`, variantH, []byte(`["string",42,{"key":"value"},[0,1]]`)},
		// Wildcard dot notation on object
		{`$.*`, variantI, []byte(`["string",42,{"key":"value"},[0,1]]`)},
	}

	for _, tst := range tests {
		prev := append(tst.Base[:0:0], tst.Base...)
		res, err := Get(tst.Base, tst.Query)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if compareSlices(res, tst.Expected) != 0 {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(res) + "`")
		}
		if !bytes.Equal(prev, tst.Base) {
			t.Errorf("Source json modified")
		}
	}
}

func Test_Fixes(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected []byte
	}{
		// closing square bracket inside a string value has been mistakenly seen as an array bound
		{[]byte(`{"foo":["[]"],"bar":123}`), `$.bar`, []byte(`123`)},
		// escaped backslash at the end of string caused parser to miss the end of string
		{[]byte(`{"foo":"\\","bar":123}`), `$.bar`, []byte(`123`)},
	}

	for _, tst := range tests {
		res, err := Get(tst.Data, tst.Query)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if compareSlices(res, tst.Expected) != 0 {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(res) + "`")
		}
	}
}

func Test_Errors(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected string
		Result   []byte
	}{
		/* normally only . and [ expected after the key
		{data, `$.store(foo`, `path: invalid character at 7`, []byte{}},
		// unexpected EOF before :
		{[]byte(`{"foo"  `), `$.foo`, `unexpected end of input`, []byte{}},
		// unexpected EOF after :
		{[]byte(`{"foo" : `), `$.foo`, `unexpected end of input`, []byte{}},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo[0]`, `unexpected end of input`, []byte{}},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo[0].bar`, `unexpected end of input`, []byte{}},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo.bar`, `unexpected end of input`, []byte{}},
		// wrong type
		{[]byte(`["foo" : ("bar")]`), `$.foo.bar`, `unrecognized value: true, false or null expected`, []byte{}},

		// start with $
		{data, `foo`, `path: $ expected`, []byte{}},
		// empty
		{data, ``, `path: empty`, []byte{}},
		// unexpected end
		{data, `$.`, `path: unexpected end of path at 2`, []byte{}},
		// bad function
		{data, `$.foo()`, `path: unknown function at 5`, []byte{}},

		// array: index bound missing
		{data, `$.store.book[1`, `path: unexpected end of path at 14`, []byte{}},
		// array: path: 0 as a second bound does not make sense
		{data, `$.store.book[1:0`, `path: unexpected end of path at 16`, []byte{}},
		// array: index bound missing (2nd)
		{data, `$.store.book[1:3`, `path: unexpected end of path at 16`, []byte{}},
		// array: index out of bounds: not an error
		{data, `$.store.book[99]`, ``, []byte(``)},
		// array: index out of bounds: not an error
		{data, `$.store.book[-99]`, ``, []byte(``)},
		// array: slice indexes out of bounds: not an error
		{data, `$.store.book[-99:-15]`, ``, []byte(`[]`)},
		*/
		// filter expression: empty
		{data, `$.store.book[?()]`, `empty filter`, []byte{}},
		// filter expression: invalid
		{data, `$.store.book[?(1+)]`, `not enough arguments`, []byte{}},

		// wrong bool value
		{[]byte(`{"foo": Troo}`), `$.foo`, `unrecognized value: true, false or null expected`, []byte{}},
		// wrong value
		{[]byte(`{"foo": moo}`), `$.foo`, `unrecognized value: true, false or null expected`, []byte{}},
		// unexpected EOF
		{[]byte(`{"foo": { "bar": "bazz"`), `$.bar`, `unexpected end of input`, []byte{}},
		// unexpected EOF
		{[]byte(`{"foo": { "bar": 0`), `$.foo.bar`, `unexpected end of input`, []byte{}},
		// unexpected EOF
		{[]byte(`{"foo": {"bar":"moo`), `$.foo.moo`, `unexpected end of input`, []byte{}},

		// invalid json
		{[]byte(`{"foo" - { "bar": 0 }}`), `$.foo.bar`, `':' expected`, []byte{}},

		// invalid string operator
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.foo[?(@.bar > "zzz")]`, `operator is not applicable to strings`, []byte{}},
		// unknown token
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.foo[?(@.bar == 2^3)]`, `unknown token at 16`, []byte{}},

		// empty key
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.['']`, `empty key`, []byte{}},
	}

	for _, tst := range tests {
		res, err := Get(tst.Data, tst.Query)
		if err == nil {
			if !bytes.EqualFold(res, tst.Result) {
				t.Errorf(tst.Query + " : `" + string(tst.Result) + "` expected, `" + string(res) + "` received")
			}
		} else if err.Error() != tst.Expected {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(err.Error()) + "`")
		}
	}
}

func NoTest_ArraySlice(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected [][]byte
	}{
		// closing square bracket inside a string value has been mistakenly taken as an array bound
		{condensed, `$.store.bicycle.equipment[0]`, [][]byte{
			[]byte(`["paddles", "umbrella", "horn"]`),
		}},
		{condensed, `$.store.bicycle.equipment[0,2]`, [][]byte{
			[]byte(`["paddles", "umbrella", "horn"]`),
			[]byte(`["light saber", "apparel"]`),
		}},
		{condensed, `$.store.bicycle.equipment[-1]`, [][]byte{
			[]byte(`["\"quoted\""]`),
		}},
		{condensed, `$.store.book[:]`, [][]byte{
			[]byte(`{"category":"reference", "author":"Nigel Rees", "title":"Sayings of the Century", "price":8.95}`),
			[]byte(`{"category":"fiction", "author":"Evelyn Waugh", "title":"Sword of Honour", "price":12.99}`),
			[]byte(`{"category":"fiction", "author":"Herman Melville", "title": "Moby Dick", "isbn": "0-553-21311-3", "price": 8.99}`),
			[]byte(`{"category":"fiction", "author":"J. R. R. Tolkien",	"title":"The Lord of the Rings", "isbn":"0-395-19395-8", "price":22.99}`),
		}},
		{condensed, `$.store.bicycle.equipment[1:3]`, [][]byte{
			[]byte(`["peg leg", "parrot", "map"]`),
			[]byte(`["light saber", "apparel"]`),
		}},
	}

	for _, tst := range tests {
		res, err := GetArrayElements(tst.Data, tst.Query, 2)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if len(res) != len(tst.Expected) {
			t.Errorf(tst.Query+" : result length mismatch (%d expected, %d received)", len(tst.Expected), len(res))
		} else {
			for i := range res {
				if compareSlices(res[i], tst.Expected[i]) != 0 {
					t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected[i]) + "`\n\tbut got  `" + string(res[i]) + "`")
				}
			}
		}
	}
}

func NoTest_ArraySlice_Errors(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected string
	}{
		// start with $
		{data, `foo`, `path: $ expected`},
		// empty
		{data, ``, `path: empty`},
		// unexpected end
		{data, `$.`, `path: unexpected end of path`},
		// bad function
		{data, `$.foo()`, `path: unknown function`},

		// unexpected EOF before :
		{[]byte(`   `), `$.foo`, `unexpected end of input`},
		// invalid value format
		{[]byte(`xxx`), `$.foo`, `object or array expected`},

		// gae() limitations
		{data, `$.store.*.foo`, `wildcards are not supported in GetArrayElements`},
		// gae() limitations
		{data, `$.store.length()`, `functions are not supported in GetArrayElements`},
		// gae() limitations
		{data, `$.store.book[:].foo[:]`, `sub-slicing is not supported in GetArrayElements`},

		// array index bounds
		{condensed, `$.store.bicycle.equipment[5]`, `specified array element not found`},
		// array index bounds
		{condensed, `$.store.bicycle.equipment[0:5]`, `specified array element not found`},
		// array index bounds
		{condensed, `$.store.bicycle.equipment[-8]`, `specified array element not found`},
	}

	for _, tst := range tests {
		_, err := GetArrayElements(tst.Data, tst.Query, 2)
		if err == nil {
			t.Errorf(tst.Query + " : error expected")
		} else if err.Error() != tst.Expected {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(err.Error()) + "`")
		}
	}
}

func Benchmark_Unmarshal(b *testing.B) {
	var jdata interface{}
	for i := 0; i < b.N; i++ {
		json.Unmarshal(data, &jdata)
	}
}

func Benchmark_Oliveagle_Jsonpath(b *testing.B) {
	b.StopTimer()
	var jdata interface{}
	err := json.Unmarshal(data, &jdata)
	if err != nil {
		fmt.Println(err.Error())
		return
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = jsonpath.JsonPathLookup(jdata, "$.store.book[3].title")
	}
}

func Benchmark_JsonSlice_ParsePath(b *testing.B) {
	b.StopTimer()
	path := []byte("$.store.book.thule.foo.bar")
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	nodePool.Put(nodePool.Get())
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		node, _, _ := readRef(path, 1, 0)
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
}

func Benchmark_Jsonslice_Get(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Get(data, "$.store.book[3].title")
	}
}

func Benchmark_Jsonslice_Get_Aggregated(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Get(data, "$.store.book[1:4].isbn")
	}
}

func GenerateLargeData() []byte {
	largeData := []byte(`{"store":{ "book": [`)
	book0, _ := Get(data, "$.store.book[0]")
	book1, _ := Get(data, "$.store.book[1]")
	for i := 0; i < 100000; i++ {
		largeData = append(largeData, book0...)
		largeData = append(largeData, ',')
	}
	largeData = append(largeData, book1...)
	largeData = append(largeData, []byte("]}")...)
	return largeData
}
func Benchmark_Unmarshal_10Mb(b *testing.B) {
	var jdata interface{}
	b.StopTimer()
	largeData := GenerateLargeData()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		json.Unmarshal(largeData, &jdata)
	}
}

/*
func Benchmark_Oliveagle_Jsonpath_10Mb_First(b *testing.B) {
	b.StopTimer()
	var jdata interface{}
	largeData := GenerateLargeData()
	json.Unmarshal(largeData, &jdata)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = jsonpath.JsonPathLookup(jdata, "$.store.book[0].title")
	}
}

func Benchmark_Oliveagle_Jsonpath_10Mb_Last(b *testing.B) {
	b.StopTimer()
	var jdata interface{}
	largeData := GenerateLargeData()
	json.Unmarshal(largeData, &jdata)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = jsonpath.JsonPathLookup(jdata, "$.store.book[100000].title")
	}
}
*/
func Benchmark_Jsonslice_Get_10Mb_First(b *testing.B) {
	b.StopTimer()
	largeData := GenerateLargeData()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get(largeData, "$.store.book[0].title")
	}
}

func Benchmark_Jsonslice_Get_10Mb_Last(b *testing.B) {
	b.StopTimer()
	largeData := GenerateLargeData()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Get(largeData, "$.store.book[100000].title")
	}
}
