[![Build Status](https://travis-ci.org/bhmj/jsonslice.svg?branch=master)](https://travis-ci.org/bhmj/jsonslice)
[![Go Report Card](https://goreportcard.com/badge/github.com/bhmj/jsonslice)](https://goreportcard.com/report/github.com/bhmj/jsonslice)
[![cover.run](https://cover.run/go/github.com/bhmj/jsonslice.svg?style=flat&tag=golang-1.10)](https://cover.run/go?tag=golang-1.10&repo=github.com%2Fbhmj%2Fjsonslice)
[![GoDoc](https://godoc.org/github.com/bhmj/jsonslice?status.svg)](https://godoc.org/github.com/bhmj/jsonslice)


# JSON Slice

## What is it?

JsonSlice is a Go package which allows to execute fast jsonpath queries without unmarshalling the whole data.  

Sometimes you need to get a single value from incoming json using jsonpath, for example to route data accordingly or so. To do that you must unmarshall the whole data into interface{} struct and then apply some jsonpath library to it, only to get just a tiny little value. What a waste of resourses! Well, now there's `jsonslice`.

Simply call `jsonslice.Get` on your raw json data to slice out just the part you need. The `[]byte` received can then be unmarshalled into a struct or used as it is.

## Getting started

#### 1. install

```
$ go get github.com/bhmj/jsonslice
```

#### 2. use it

```
import "github.com/bhmj/jsonslice"
import "fmt"

func main() {
  var data = []byte(`
    { "arr": [ 
        { "elem": {"text": "hi!"} } 
      ]
    }
  `)

  v, err := jsonslice.Get(data, "$.arr[0].elem")

  fmt.Println(string(v)) // {"text": "hi!"}
}
```

## Benchmarks

```diff
$ go test -bench=. -benchmem -benchtime=4s
goos: windows
goarch: amd64
pkg: github.com/bhmj/jsonslice
++ here's a couple of operations usually needed to get an object by jsonpath (for reference):
Benchmark_Unmarshal-4                   200000        23321 ns/op    3568 B/op    88 allocs/op
Benchmark_Oliveagle_Jsonpath-4         1000000         4160 ns/op     608 B/op    48 allocs/op
++ here's a couple of operations needed to get a jsonslice:
Benchmark_JsonSlice_ParsePath-4        5000000         1152 ns/op     432 B/op     9 allocs/op
Benchmark_Jsonslice_Get-4              1000000         4563 ns/op     432 B/op     9 allocs/op
++ in case you aggregate some non-contiguous elements, it may take a bit longer (extra mallocs involved):
Benchmark_Jsonslice_Get_Aggregated-4   1000000         9030 ns/op    2101 B/op    19 allocs/op
++ unmarshalling a large json:
Benchmark_Unmarshal_10Mb-4                  50    107806168 ns/op     376 B/op     5 allocs/op
++ jsonslicing the same json, target element is near the start:
Benchmark_Jsonslice_Get_10Mb_First-4   2000000         2421 ns/op     432 B/op     9 allocs/op
++ jsonslicing the same json, target element is near the end: still beats Unmarshal
Benchmark_Jsonslice_Get_10Mb_Last-4        100     59093380 ns/op     432 B/op     9 allocs/op
PASS
ok      github.com/bhmj/jsonslice       82.348s
```

## Specs

See [Stefan Gössner's article](http://goessner.net/articles/JsonPath/index.html#e2) for original specs and examples.  

## Limitations and deviations

1. Only single-word keys (`/\w+/`) are supported by now. 

2. Only dot notation (`$.foo.bar`) is supported by now. Bracket notation is coming soon.

3. A single index reference returns an element, not an array:  
```
./jsonslice sample0.json $.store.book[0]
```
returns  
```
{
  "category": "reference",
  "author": "Nigel Rees",
  "title": "Sayings of the Century",
  "price": 8.95
}
```
while this query
```
./jsonslice sample0.json $.store.book[0:1]
```
returns an array 
```
[{
  "category": "reference",
  "author": "Nigel Rees",
  "title": "Sayings of the Century",
  "price": 8.95
}]
```

Also, indexing on root node is supported (assuming json is an array and not an object):  
```
./jsonslice sample1.json $[0].author
```

## Expressions

### Common expressions

#### Operators 
```
  $                   -- root node (can be either object or array)
  .node               -- dot-notated child
  [123]               -- array index
  [12:34]             -- array range
```
#### Functions
```
  $.obj.length()      -- number of elements in an array or string length, depending on the obj type
  $.obj.count()       -- same as above
  $.obj.size()        -- object size in bytes (as is)
```
#### Objects
```
  $.obj
  $.obj.val
```
####  Indexed arrays
```
  $.obj[3]
  $.obj[3].val
  $.obj[-2]  -- second from the end
```
#### Ranged arrays
```
  $.obj[:]   -- == $.obj (all elements of the array)
  $.obj[0:]  -- the same as above: items from index 0 (inclusive) till the end
  $.obj[<anything>:0] -- doesn't make sense (from some element to the index 0 exclusive -- which is always empty)
  $.obj[2:]  -- items from index 2 (inclusive) till the end
  $.obj[:5]  -- items from the beginning to index 5 (exclusive)
  $.obj[-2:] -- items from the second element from the end (inclusive) till the end
  $.obj[:-2] -- items from the beginning to the second element from the end (exclusive, i.e. without two last elements)
  $.obj[:-1] -- items from the beginning to the end but without one final element
  $.obj[3:5] -- items from index 2 (inclusive) to index 5 (exclusive)
```

### Aggregating expressions

#### Sub-querying
```
  $.obj[any:any].something  -- composite sub-query
  $.obj[3,5,7]              -- multiple array indexes
```
#### Filters
```
  @                  -- the current node
  [?(<expression>)]  -- filter expression. Applicable to arrays only
```

#### Filter operators

  Operator | Description
  --- | ---
  `==`  | Equal to<br>Use single or double quotes for string expressions.<br>`[?(@.color=='red')]` or `[?(@.color=="red")]`
  `!=`  | Not equal to
  `>`   | Greater than
  `>=`  | Grater than or equal to
  `<`   | Less than
  `<=`  | Less than or equal to
  `=~`  | Match a regexp<br>`[?(@.name =~ /cat.*/i]`
  `&&`  | Logical AND
  `\|\|`  | Logical OR

"Having" filter:  
`$.stores[?(@.work_time[:].time_close=="16:00:00")])].id` -- find IDs of every store having at least one day with a closing time at 16:00

### Updates (TODO)

```
  $.obj[?(@.price > 1000)].expensive = true                    -- add/replace field value
  $.obj[?(@.authors.size() > 2)].title += " (multi authored)"  -- expand field value
  $.obj[?(@.price > $.expensive)].bonus = $.bonuses[0].value   -- add/replace field using another jsonpath 
```

## Examples

  Assuming `sample0.json` and `sample1.json` in the example directory:  

  `./jsonslice sample0.json '$.store.book[0]'`  
  `./jsonslice sample0.json '$.store.book[0].title'`  
  `./jsonslice sample0.json '$.store.book[0:-1]'`  
  `./jsonslice sample1.json '$[1].author'`  
  `./jsonslice sample0.json '$.store.book[?(@.price > 10)]'`  
  `./jsonslice sample0.json '$.store.book[?(@.price > $.expensive)]'`  
  
## Changelog

**0.6.3** (2018-07-16) -- Boolean/null value error fixed.

**0.6.2** (2018-07-03) -- More tests added, error handling clarified.

**0.6.1** (2018-06-26) -- Nested array indexing is now supported.
> `$.store.bicycle.equipment[1][0]` -> `"peg leg"`

**0.6.0** (2018-06-25) -- Regular expressions added.
> `$.store.book[?(@.title =~ /(dick)|(lord)/i)].title` -> `["Moby Dick","The Lord of the Rings"]`

**0.5.1** (2018-06-15) -- Logical expressions added.
> `$.store.book[?(@.price > $.expensive && @.isbn)].title` -> `["The Lord of the Rings"]`

**0.5.0** (2018-06-14) -- Expressions added.
> `$.store.book[?(@.price > $.expensive)].title` -> `["Sword of Honour","The Lord of the Rings"]`

**0.4.0** (2018-05-16) -- Aggregating sub-queries added.
> `$.store.book[1:3].author` -> `["John","William"]`

**0.3.0** (2018-05-05) -- Beta  

## Roadmap

- [x] length(), count(), size() functions
- [x] filters: simple expressions
- [x] filters: complex expressions (with logical operators)
- [x] nested arrays support
- [ ] deepscan operator (`..`)
- [ ] bracket notation for multiple field queries
- [ ] assignment in query (update json)

## Contributing

1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :)

## Licence

[MIT](http://opensource.org/licenses/MIT)

## Author

Michael Gurov aka BHMJ
