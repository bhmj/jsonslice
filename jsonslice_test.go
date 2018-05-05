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

func NoTestGet(t *testing.T) {
	res, err := Get(data, "$.expensive")
	if compareSlices(res, []byte("10")) != 0 && err == nil {
		t.Errorf("expensive should be 10, but got \"" + string(res) + "\"")
	}
	res, err = Get(data, "$.store.book[3].author")
	if compareSlices(res, []byte(`"J. R. R. Tolkien"`)) != 0 && err == nil {
		t.Errorf("store.book[3].author should be \"J. R. R. Tolkien\", but got \"" + string(res) + "\"")
	}
}

func BenchmarkPath(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = parsePath([]byte("$.store.book[3].title"))
	}
}

func BenchmarkGet(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_, _ = Get(data, "$.store.book[3].title")
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for i := 0; i < b.N; i++ {
		var jdata interface{}
		json.Unmarshal(data, &jdata)
	}
}

func BenchmarkJsonpath(b *testing.B) {
	var jdata interface{}
	json.Unmarshal(data, &jdata)
	for i := 0; i < b.N; i++ {
		_, _ = jsonpath.JsonPathLookup(jdata, "$.store.book[3].title")
	}
}
