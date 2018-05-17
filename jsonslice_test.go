package jsonslice

import (
	"encoding/json"
	"testing"

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

func compareSlices(s1 []byte, s2 []byte) int {
	if len(s1) != len(s2) {
		return len(s1) - len(s2)
	}
	for i := range s1 {
		if s1[i] != s2[i] {
			return int(s1[i] - s2[i])
		}
	}
	return 0
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
