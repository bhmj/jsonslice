package jsonslice

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var data []byte
var condensed []byte
var differentTypes []byte
var rfc3339Data []byte

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
	differentTypes = []byte(`
		[
			{"key": "some"},
			{"key": "value"},
			{"key": null},
			{"key": true},
			{"key": false},
			{"key": 0},
			{"key": 1},
			{"key": -1},
			{"key": ""},
			{"key": "0"},
			{"key": "1"},
			{"key": {}},
			{"key": []},
			{"key": "valuemore"},
			{"key": "morevalue"},
			{"key": ["value"]},
			{"key": {"some": "value"}},
			{"key": {"key": "value"}},
			{"some": "value"},
			{"key": 42},
			{"key": 41},
			{"key": 43},
			{"key": 42.0001},
			{"key": 41.9999},
			{"key": "42"},
			{"key": 420},
			{"key": [42]},
			{"key": {"key": 42}},
			{"key": {"some": 42}}
	  	]
	`)
	rfc3339Data = []byte(`
		{
			"store": {
				"open": true,
				"book": [
					{"category":"reference", "author":"Nigel Rees", "title":"Sayings of the Century", "price":8.95, "date":"2022-09-18T07:25:40.20Z"},
					{"category":"fiction", "author":"Evelyn Waugh", "title":"Sword of Honour", "price":12.99, "date":"2022-09-18"}
				],
				"founded": "2022-09-18T07:25:40.20Z",
				"random": "2022-02-20"
			}
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
		_, _, _ = readRef([]byte(str), 1, 0)
	}
	fmt.Println()
}

func TestCustomPath(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	_, _, _ = readRef([]byte("$.()"), 1, 0)
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
		_, _ = Get([]byte(str), "$.some.value")
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
		{`$.store.book[?(@.price >= $.expensive || @.isbn)].title`, []byte(`["Sword of Honour","Moby Dick","The Lord of the Rings"]`)},
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

		// regexp: simple
		{`$.store.book[?(@.title =~ /the/i)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},
		// regexp: complex
		{`$.store.book[?(@.title =~ /(Saying)|(Lord)/)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},
		// regexp: not equal
		{`$.store.book[?(@.title !=~ /(Saying)|(Lord)/)].title`, []byte(`["Sword of Honour","Moby Dick"]`)},
		// regexp: same as above plus syntax variation: != or !=~
		{`$.store.book[?(@.title !=~ /saying/i && @.title !~ /Lord/)].title`, []byte(`["Sword of Honour","Moby Dick"]`)},

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
		// wildcard: named key in any array (sliced) on a given level
		{`$.store.*[1:3].price`, []byte(`[12.99,8.99]`)},

		// all elements of an empty array is an empty array
		{`$.store.manager[:]`, []byte(`[]`)},
		// multiple keys (output ordered as in data)
		{`$.store.book[:]['price','title']`, []byte(`[["Sayings of the Century",8.95],["Sword of Honour",12.99],["Moby Dick",8.99],["The Lord of the Rings",22.99]]`)},
		// multiple keys over a sliced array
		{`$.store.book[1:3]['price','title']`, []byte(`[["Sword of Honour",12.99],["Moby Dick",8.99]]`)},
		// multiple keys combined with filter
		{`$.store.book[?(@.price > $.expensive*1.1)]['price','title']`, []byte(`["Sword of Honour",12.99,"The Lord of the Rings",22.99]`)},
		// functions in filter
		{`$.store.bicycle.equipment[?(@.count() == 2)][1]`, []byte(`["apparel"]`)},
	}

	for _, tst := range tests {
		// println(tst.Query)
		res, err := Get(data, tst.Query)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if compareSlices(res, tst.Expected) != 0 {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(res) + "`")
		}
	}
}

func Test_FuncNow(t *testing.T) {
	tests := []struct {
		name           string
		Query          string
		expectedOutput func(t *testing.T) ([]byte, error)
	}{
		{"happy path", "$.now()", expectedSuccessfulFuncNow},
		{"invalid use of now function, will return nil", "$.x.now()", expectedFailFuncNow},
		{"invalid use of now function, will return nil root level", "$.now(\"2006-01-02T15:04:05.000Z\")", expectedFailFuncNow},
	}

	for _, tst := range tests {
		// println(tst.Query)
		actual, _ := Get(data, tst.Query)
		expected, err := tst.expectedOutput(t)
		if err != nil {
			if actual != nil {
				t.Errorf("\n\ttestName:" + tst.name + "testQuery:\n\t" + tst.Query + "\n\texpected `" + string("<nil>") + "`\n\tbut got  `" + string(actual) + "`")
			}
		} else {
			if compareSlices(actual, expected) != 0 {
				t.Errorf("\n\ttestName:" + tst.name + "\n\ttestQuery:" + tst.Query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(actual) + "`")
			}
		}
	}
}

func Test_FuncNowRFC3339(t *testing.T) {
	tests := []struct {
		name           string
		Query          string
		testInput      string
		expectedOutput func(t *testing.T, s string) ([]byte, error)
	}{
		{"happy path", "$.now().RFC3339()", "", func(t *testing.T, s string) ([]byte, error) {
			t.Helper()
			tt, err := time.Parse(time.RFC3339, time.Now().Format("2006-01-02T15:04:05.000Z"))
			if err != nil {
				return nil, err
			}
			expected, _ := json.Marshal(tt)

			return expected, nil
		}},
		{"happy path with date node", "$.store.founded.RFC3339()", "2022-09-18T07:25:40.20Z", func(t *testing.T, s string) ([]byte, error) {
			t.Helper()
			tt, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return nil, err
			}
			expected, _ := json.Marshal(tt)

			return expected, nil
		}},
		{"invalid input node, will return an empty array", "$.store.book[?(@.category==\"fiction\")].date.RFC3339()", "", func(t *testing.T, s string) ([]byte, error) {
			t.Helper()
			return []byte("[]"), nil
		}},
		{"invalid input node, will return an empty string", "$.store.random.RFC3339()", "", func(t *testing.T, s string) ([]byte, error) {
			t.Helper()
			return []byte(""), nil
		}},
		{"invalid input node, will return an empty string", "$.RFC3339()", "", func(t *testing.T, s string) ([]byte, error) {
			t.Helper()
			return nil, errors.New("RFC3339() is only applicable to string date that can be formatted to RFC3339")
		}},
	}

	for _, tst := range tests {
		// println(tst.Query)
		actual, actualErr := Get(rfc3339Data, tst.Query)
		expected, err := tst.expectedOutput(t, tst.testInput)
		if actualErr != nil && err == nil {
			t.Errorf("testName:%s\n,testQuery:%s\nunexepectedError:%v", tst.name, tst.Query, actualErr)
		}

		if err != nil {
			if actual != nil {
				t.Errorf("testName:%s\n,testQuery:%s\nexpected:<nil> but got %s", tst.name, tst.Query, string(actual))
			}
		} else if err != nil {
			if actualErr != err {
				t.Errorf("testName:%s\n,testQuery:%s\nexpectedErr:%v but got %v", tst.name, tst.Query, err, actualErr)
			}
		} else {
			if compareSlices(actual, expected) != 0 {
				t.Errorf("testName:%s\n,testQuery:%s\nexpected:%s but got %s", tst.name, tst.Query, string(expected), string(actual))
			}
		}
	}
}

// expectedSuccessfulFuncNow return expected output for successful function time now()
func expectedSuccessfulFuncNow(t *testing.T) ([]byte, error) {
	t.Helper()
	tt := time.Now().Format("2006-01-02T15:04:05.000Z")
	expected, _ := json.Marshal(tt)

	return expected, nil
}

// expectedFailFuncNow return expected output for failed function time now()
func expectedFailFuncNow(t *testing.T) ([]byte, error) {
	return nil, errors.New("should return nil")
}

func Test_AbstractComparison(t *testing.T) {

	tests := []struct {
		Query    string
		Expected []byte
	}{
		// abstract integer
		{`$[?(@.key == 1)]`, []byte(`[{"key": true},{"key": 1},{"key": "1"}]`)},
		// abstract integer
		{`$[?(@.key == 0)]`, []byte(`[{"key": false},{"key": 0},{"key": ""},{"key": "0"}]`)},
		// strict integer
		{`$[?(@.key === 1)]`, []byte(`[{"key": 1}]`)},
		// strict integer
		{`$[?(@.key === 0)]`, []byte(`[{"key": 0}]`)},
		// abstract string
		{`$[?(@.key == "1")]`, []byte(`[{"key": true},{"key": 1},{"key": "1"}]`)},
		// abstract string
		{`$[?(@.key == "0")]`, []byte(`[{"key": false},{"key": 0},{"key": "0"}]`)},
		// strict string
		{`$[?(@.key === "0")]`, []byte(`[{"key": "0"}]`)},
		// abstract boolean
		{`$[?(@.key == true)]`, []byte(`[{"key": true},{"key": 1},{"key": "1"}]`)},
		// abstract boolean
		{`$[?(@.key == false)]`, []byte(`[{"key": false},{"key": 0},{"key": ""},{"key": "0"}]`)},
		// strict boolean
		{`$[?(@.key === true)]`, []byte(`[{"key": true}]`)},
		// strict boolean
		{`$[?(@.key === false)]`, []byte(`[{"key": false}]`)},
		// cburgmer's string
		{`$[?(@.key === "value")]`, []byte(`[{"key": "value"}]`)},
		// abstract number (int)
		{`$[?(@.key==42)]`, []byte(`[{"key": 42},{"key": "42"}]`)},
		// strict number (int)
		{`$[?(@.key===42)]`, []byte(`[{"key": 42}]`)},
		// strict number (float)
		{`$[?(@.key===42.0)]`, []byte(`[{"key": 42}]`)},
	}

	for _, tst := range tests {
		// println(tst.Query)
		res, err := Get(differentTypes, tst.Query)
		if err != nil {
			t.Errorf(tst.Query + " : " + err.Error())
		} else if compareSlices(res, tst.Expected) != 0 {
			t.Errorf(tst.Query + "\n\texpected `" + string(tst.Expected) + "`\n\tbut got  `" + string(res) + "`")
		}
	}
}

func Test_StringComparison(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected []byte
	}{
		// string comparison
		{[]byte(`{"foo":[{"key":"moo"}]}`), `$.foo[?(@.key > "zzz")]`, []byte(`[]`)},
		// string comparison
		{[]byte(`{"foo":[{"key":"moo"}]}`), `$.foo[?(@.key > "abc")]`, []byte(`[{"key":"moo"}]`)},
		// string comparison
		{[]byte(`{"foo":[{"key":"12"}]}`), `$.foo[?(@.key > "2")]`, []byte(`[]`)},
		// string and number comparison
		{[]byte(`{"foo":[{"key":"12"}]}`), `$.foo[?(@.key > 2)]`, []byte(`[{"key":"12"}]`)},
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

func Test_Extensions(t *testing.T) {

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
	variantK := []byte(`[{"key": [{"baz": ["hello"]}], "foo": [{"key": ["eee!", {"key":[{"ouch":2}]}]}]}]`)
	variantL := []byte(`{"a":{"b":[{"c":"cc1","d":"dd1"},{"c":"cc2","d":"dd2"}]}}`)

	tests := []struct {
		Query    string
		Base     []byte
		Expected []byte
	}{
		// custom extensions
		// {`$.'book'[1]`, variant1, []byte(`{"Author": "Z.Hopp", "Title": "Trollkrittet"}`)},
		// {`$.'book'.1`, variant1, []byte(`{"Author": "Z.Hopp", "Title": "Trollkrittet"}`)},
		// wildcard in key list ignored if not alone
		{`$.[0,*,-1]`, variant4, []byte(`["first","third"]`)},

		// gold standard

		// array index dot notation
		{`$.book.2`, variant2, []byte(`{"Book three"}`)},
		// array index dot notation on object
		{`$.2`, variant3, []byte(`"second"`)},
		// array index slice end out of bounds
		{`$[1:10]`, variant4, []byte(`["second", "third"]`)},
		// array index slice negative step
		{`$[::-2]`, variant5, []byte(`["fifth","third","first"]`)},
		// Array index slice start end negative step
		{`$[3:0:-2]`, variant5, []byte(`["fourth","second"]`)},
		// Array index slice start end step
		{`$[0:3:2]`, variant5, []byte(`["first","third"]`)},
		// Array index slice start end step 0
		{`$[0:3:0]`, variant5, []byte(`["first", "second", "third"]`)},
		// Array index slice start end step 1
		{`$[0:3:1]`, variant5, []byte(`["first", "second", "third"]`)},
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
		{`$[*].bar[*].baz`, variantJ, []byte(`[["hello"]]`)},
		// Wildcard dot notation on array
		{`$.*`, variantH, []byte(`["string",42,{"key":"value"},[0,1]]`)},
		// Wildcard dot notation on object
		{`$.*`, variantI, []byte(`["string",42,{"key":"value"},[0,1]]`)},

		// variations
		{`$..key..[0]`, variantK, []byte(`[[{"baz": ["hello"]},"hello"],["eee!",{"ouch":2}],[{"ouch":2}]]`)},
		{`$.a.b[:].['c','d']`, variantL, []byte(`[["cc1","dd1"],["cc2","dd2"]]`)},
	}

	for _, tst := range tests {
		prev := append(tst.Base[:0:0], tst.Base...)
		// println(tst.Query)
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
		// using indexing of array element inside expression
		// fixed in 1.1.1
		{[]byte(`[ [2,3], ["a"], [0,2], [2] ]`), `$[?(@[-1]==2)]`, []byte(`[[0,2],[2]]`)},
		// single/double quoted notation with backslash escaped backslash
		// fixed in 1.1.1
		{[]byte(`{"\\": "value"}`), `$['\\']`, []byte(`"value"`)},
		{[]byte(`{"\\": "value"}`), `$["\\"]`, []byte(`"value"`)},
		// closing square bracket inside a string value has been mistakenly seen as an array bound
		// fixed in 0.7.2
		{[]byte(`{"foo":["[]"],"bar":123}`), `$.bar`, []byte(`123`)},
		// escaped backslash at the end of string caused parser to miss the end of string
		// fixed in 1.0.3
		{[]byte(`{"foo":"foo \\","bar":123}`), `$.foo`, []byte(`"foo \\"`)},
		// escaped backslash at the end of string caused parser to miss the end of string
		// fixed in 1.0.3
		{[]byte(`[{"foo":"foo \\","bar":123}]`), `$[0].foo`, []byte(`"foo \\"`)},
		// https://github.com/bhmj/jsonslice/issues/12
		// $+ is obviously invalid but acts as simple $ now
		// fixed in 1.0.4
		{[]byte(`[{"foo":"bar"}]`), `$+`, []byte(`[{"foo":"bar"}]`)},
		// https://github.com/bhmj/jsonslice/issues/15
		// "$..many.keys" used to trigger on "many" without recursing deeper on "keys"
		// fixed in 1.0.5
		{[]byte(`{"kind":"Pod", "spec":{ "containers": [{"name":"c1"}, {"name":"c2"}] }}`), `$.spec.containers[:]`, []byte(`[{"name":"c1"},{"name":"c2"}]`)},
		{[]byte(`{"kind":"Pod", "spec":{ "containers": [{"name":"c1"}, {"name":"c2"}] }}`), `$..spec.containers[:]`, []byte(`[[{"name":"c1"},{"name":"c2"}]]`)},
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

func Test_Unicode(t *testing.T) {

	tests := []struct {
		Data     []byte
		Query    string
		Expected []byte
	}{
		// json: utf-8, path: utf-8
		{[]byte(`{"Motörhead":"Lemmy"}`), `$."Motörhead"`, []byte(`"Lemmy"`)},
		// json: utf-8, path: unicode escaped
		{[]byte(`{"Motörhead":"Lemmy"}`), `$."Mot\u00F6rhead"`, []byte(`"Lemmy"`)},
		// json: unicode escaped, path: utf-8
		{[]byte(`{"Mot\u00F6rhead":"Lemmy"}`), `$."Motörhead"`, []byte(`"Lemmy"`)},
		// json: & path: both unicode escaped
		{[]byte(`{"Mot\u00F6rhead":"Lemmy"}`), `$."Mot\u00F6rhead"`, []byte(`"Lemmy"`)},
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
		// normally only . and [ expected after the key
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
		{[]byte(`{"foo": {"bar":"moo`), `$.foo.moo`, `unexpected end of input`, []byte{}},

		// invalid json
		{[]byte(`{"foo" - { "bar": 0 }}`), `$.foo.bar`, `':' expected`, []byte{}},

		// unknown token
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.foo[?(@.bar == 2#3)]`, `unknown token at 8: #3 at 8`, []byte{}},

		// empty key
		{[]byte(`{"foo":[{"bar":"moo"}]}`), `$.['']`, `empty key`, []byte{}},

		// Key bracket notation with single quote
		{[]byte(`{"single'quote":"value"}`), `$['single'quote']`, `path: invalid character at 10`, []byte{}},

		// unexpected EOF
		// NOTE:
		// The following json technically is incorrect but due to optimization techniques used it is processed successfully.
		// This way of "lazy" processing may be fixed in the future.
		// {[]byte(`{"foo": { "bar": 0`), `$.foo.bar`, `unexpected end of input`, []byte{}},
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

func Benchmark_Unmarshal(b *testing.B) {
	var jdata interface{}
	for i := 0; i < b.N; i++ {
		_ = json.Unmarshal(data, &jdata)
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

func Benchmark_Jsonslice_Get_Simple(b *testing.B) {
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
		_ = json.Unmarshal(largeData, &jdata)
	}
}

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

// helpers

func compareSlices(s1 []byte, s2 []byte) int {
	if len(s1)+len(s2) == 0 {
		return 0
	}
	i := 0
	for i = 0; i < len(s1); i++ {
		if i > len(s2)-1 {
			return 1
		}
		if s1[i] != s2[i] {
			return int(s1[i]) - int(s2[i])
		}
	}
	if i < len(s2) {
		return -1
	}
	return 0
}
