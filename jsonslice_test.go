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

func init() {
	data = []byte(`
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
					"price": 19.95,
					"equipment": [
						["paddles", "umbrella", "horn"],
						["peg leg", "parrot", "map"],
						["light saber", "apparel"]
					]
				}
			},
			"expensive": 10
		}
	`)
}

func TestFuzzy(t *testing.T) {
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
	fmt.Printf("\r[                    ]\r[")
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

func TestFuzzyPath(t *testing.T) {
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
	for i := 0; i < 10000000; i++ {
		n, err := rand.Read(b[:rand.Int()%len(b)])
		if err != nil {
			t.Fatal(err)
		}
		str = string(b[:n])
		parsePath([]byte(str))
	}
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
		// simple query
		{`$.expensive`, []byte(`10`)},
		// simple query
		{`$.store.book[3].author`, []byte(`"J. R. R. Tolkien"`)},

		// aggregated
		{`$.store.book[1:3].author`, []byte(`["Evelyn Waugh","Herman Melville"]`)},
		// aggregated, skip missing keys
		{`$.store.book[1:].isbn`, []byte(`["0-553-21311-3","0-395-19395-8"]`)},
		// aggregated, enumerate indexes
		{`$.store.book[0,2].title`, []byte(`["Sayings of the Century","Moby Dick"]`)},

		// simple expression
		{`$.store.book[?(@.price>10)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
		// simple expression
		{`$.store.book[?(@.price==12.99)].title`, []byte(`["Sword of Honour"]`)},
		// +spaces
		{`$.store.book[?(@.price > 10)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
		// field presence
		{`$.store.book[?(@.isbn)].title`, []byte(`["Moby Dick","The Lord of the Rings"]`)},
		// string match
		{`$.store.book[?(@.isbn == "0-553-21311-3")].title`, []byte(`["Moby Dick"]`)},
		// string mismatch
		{`$.store.book[?(@.isbn != "0-553-21311-3")].title`, []byte(`["The Lord of the Rings"]`)},
		// root references
		{`$.store.book[?(@.price > $.expensive)].title`, []byte(`["Sword of Honour","The Lord of the Rings"]`)},
		// math
		{`$.store.book[?(@.price > $.expensive*2)].title`, []byte(`["The Lord of the Rings"]`)},
		// logic operators : AND
		{`$.store.book[?(@.price > $.expensive && @.isbn)].title`, []byte(`["The Lord of the Rings"]`)},
		// logic operators : OR
		{`$.store.book[?(@.price >= $.expensive || @.isbn)].title`, []byte(`["Moby Dick","The Lord of the Rings"]`)},

		// regexp
		{`$.store.book[?(@.title =~ /the/i)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},
		// regexp
		{`$.store.book[?(@.title =~ /(Saying)|(Lord)/)].title`, []byte(`["Sayings of the Century","The Lord of the Rings"]`)},

		// array of arrays
		{`$.store.bicycle.equipment[1][0]`, []byte(`"peg leg"`)},
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

func Benchmark_Unmarshal(b *testing.B) {
	var jdata interface{}
	for i := 0; i < b.N; i++ {
		json.Unmarshal(data, &jdata)
	}
}

func Benchmark_Oliveagle_Jsonpath(b *testing.B) {
	b.StopTimer()
	var jdata interface{}
	json.Unmarshal(data, &jdata)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = jsonpath.JsonPathLookup(jdata, "$.store.book[3].title")
	}
}

func Benchmark_JsonSlice_ParsePath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = parsePath([]byte("$.store.book[3].title"))
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
