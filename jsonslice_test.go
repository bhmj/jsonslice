package jsonslice

import (
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
	top := 1000000
	fmt.Printf("\rpath fuzzy [                    ]\rpath fuzzy [")
	for i := 0; i < top; i++ {
		if i%(top/20) == 1 {
			fmt.Printf(".")
		}
		randomBytes(b, 32, 127)
		n := rand.Intn(len(b))
		b[0] = '$'
		str = string(b[:n])
		parsePath([]byte(str))
	}
	fmt.Println()
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
		{`$.store.book[?(@.price < 10)].title`, []byte(`["Sayings of the Century","Moby Dick"]`)},
		// filters: simple expression
		{`$.store.book[?(@.price==12.99)].title`, []byte(`["Sword of Honour"]`)},
		// more spaces
		{`$.store.book[?(   @.price   >   10)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
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
		{`$.store.book[?($.store.open < true)].price`, []byte(``)},
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
		{`$.store.book[?($.store[0] > 0)]`, []byte(`[]`)},
		// wildcard: any key within an object
		{`$.store.book[0].*`, []byte(`["reference","Nigel Rees","Sayings of the Century",8.95]`)},
		// wildcard: named key on a given level
		{`$.store.*.price`, []byte(`[19.95]`)},
		// wildcard: named key in any array on a given level
		{`$.store.*[:].price`, []byte(`[8.95,12.99,8.99,22.99]`)},

		// all elements of an empty array is an empty array
		{`$.store.manager[:]`, []byte(`[]`)},

		// multiple keys (ordered as in query)
		{`$.store.book[:]['price','title']`, []byte(`[[8.95,"Sayings of the Century"],[12.99,"Sword of Honour"],[8.99,"Moby Dick"],[22.99,"The Lord of the Rings"]]`)},
		// multiple keys combined with filter
		{`$.store.book[?(@.price > $.expensive*1.1)]['price','title']`, []byte(`[[12.99,"Sword of Honour"],[22.99,"The Lord of the Rings"]]`)},

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
	}{
		// normally only . and [ expected after the key
		{data, `$.store(foo`, `path: invalid element reference at 7`},
		// unexpected EOF before :
		{[]byte(`{"foo"  `), `$.foo`, `unexpected end of input`},
		// unexpected EOF after :
		{[]byte(`{"foo" : `), `$.foo`, `unexpected end of input`},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo[0]`, `array expected`},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo[0].bar`, `array expected`},
		// wrong type
		{[]byte(`{"foo" : "bar"`), `$.foo.bar`, `object expected`},
		// wrong type
		{[]byte(`["foo" : ("bar")]`), `$.foo.bar`, `object expected`},

		// start with $
		{data, `foo`, `path: $ expected`},
		// empty
		{data, ``, `path: empty`},
		// unexpected end
		{data, `$.`, `path: unexpected end of path at 2`},
		// bad function
		{data, `$.foo()`, `path: unknown function at 5`},

		// array: index bound missing
		{data, `$.store.book[1`, `path: index bound missing at 14`},
		// array: path: 0 as a second bound does not make sense
		{data, `$.store.book[1:0`, `path: 0 as a second bound does not make sense at 15`},
		// array: index bound missing (2nd)
		{data, `$.store.book[1:3`, `path: index bound missing at 16`},
		// array: node does not exist
		{data, `$.store.book[99]`, `specified array element not found`},
		// array: node does not exist
		{data, `$.store.book[-99]`, `specified array element not found`},
		// array: node does not exist
		{data, `$.store.book[-99:-15]`, `specified array element not found`},

		// filter expression: empty
		{data, `$.store.book[?()]`, `empty filter`},
		// filter expression: invalid
		{data, `$.store.book[?(1+)]`, `not enough arguments`},

		// wrong bool value
		{[]byte(`{"foo": Troo}`), `$.foo`, `unrecognized value: true, false or null expected`},
		// wrong value
		{[]byte(`{"foo": moo}`), `$.foo`, `unrecognized value: true, false or null expected`},
		// unexpected EOF
		{[]byte(`{"foo": { "bar": "bazz"`), `$.bar`, `unexpected end of input`},
		// unexpected EOF
		{[]byte(`{"foo": { "bar": 0`), `$.foo.bar`, `unexpected end of input`},
		// unexpected EOF
		{[]byte(`{"foo": {"bar":"moo`), `$.foo.moo`, `unexpected end of input`},

		// invalid json
		{[]byte(`{"foo" - { "bar": 0 }}`), `$.foo.bar`, `':' expected`},

		// invalid string operator
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.foo[?(@.bar > "zzz")]`, `operator is not applicable to strings`},
		// unknown token
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.foo[?(@.bar == 2^3)]`, `unknown token at 18`},
	}

	for _, tst := range tests {
		_, err := Get(tst.Data, tst.Query)
		if err == nil {
			t.Errorf(tst.Query + " : error expected")
		} else if err.Error() != tst.Expected {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(err.Error()) + "`")
		}
	}
}

func Test_ArraySlice(t *testing.T) {

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

func Test_ArraySlice_Errors(t *testing.T) {

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
		node, _, _ := parsePath(path)
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
