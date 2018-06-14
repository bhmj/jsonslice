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
					"price": 19.95
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

func Test_SimpleCases(t *testing.T) {
	res, err := Get(data, "$.expensive")
	if compareSlices(res, []byte("10")) != 0 && err == nil {
		t.Errorf("expensive should be 10, but got \"" + string(res) + "\"")
	}
	res, err = Get(data, "$.store.book[3].author")
	if compareSlices(res, []byte(`"J. R. R. Tolkien"`)) != 0 && err == nil {
		t.Errorf("store.book[3].author should be \"J. R. R. Tolkien\", but got \"" + string(res) + "\"")
	}
}

func Test_Aggregated(t *testing.T) {
	expected := []byte(`["Evelyn Waugh","Herman Melville"]`)
	path := "$.store.book[1:3].author"
	res, err := Get(data, path)
	if compareSlices(res, expected) != 0 && err == nil {
		t.Errorf(path + "\nexpected:\n" + string(expected) + "\ngot:\n" + string(res))
	}
	expected = []byte(`["0-553-21311-3","0-395-19395-8"]`)
	path = "$.store.book[1:].isbn"
	res, err = Get(data, path)
	if compareSlices(res, expected) != 0 && err == nil {
		t.Errorf(path + "\nexpected:\n" + string(expected) + "\ngot:\n" + string(res))
	}
	expected = []byte(`["Sayings of the Century","Moby Dick"]`)
	path = "$.store.book[0,2].title"
	res, err = Get(data, path)
	if compareSlices(res, expected) != 0 && err == nil {
		t.Errorf(path + "\nexpected:\n" + string(expected) + "\ngot:\n" + string(res))
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
	// simple expression
	expected := []byte(`["Sword of Honour","The Lord of the Rings"]`)
	query := "$.store.book[?(@.price>10)].title"
	res, err := Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
	}
	// +spaces
	expected = []byte(`["Sword of Honour","The Lord of the Rings"]`)
	query = "$.store.book[?(@.price > 10)].title"
	res, err = Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
	}
	// field presense
	expected = []byte(`["Moby Dick","The Lord of the Rings"]`)
	query = "$.store.book[?(@.isbn)].title"
	res, err = Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
	}
	// string match
	expected = []byte(`["Moby Dick"]`)
	query = `$.store.book[?(@.isbn == "0-553-21311-3")].title`
	res, err = Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
	}
	// string mismatch
	expected = []byte(`["The Lord of the Rings"]`)
	query = `$.store.book[?(@.isbn != "0-553-21311-3")].title`
	res, err = Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
	}
	// root references
	expected = []byte(`["Sword of Honour","The Lord of the Rings"]`)
	query = `$.store.book[?(@.price > $.expensive)].title`
	res, err = Get(data, query)
	if err != nil {
		t.Errorf(err.Error())
	} else if compareSlices(res, expected) != 0 {
		t.Errorf(query + "\n\texpected `" + string(expected) + "`\n\tbut got  `" + string(res) + "`")
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
