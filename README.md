# JSON Slice

## Changelog

**0.3.0** (2018-05-05) -- beta  
**0.4.0** (2018-05-16) -- aggregating sub-querying added  
> (`$.store.books[1:3].author` -> `[{"author":"John"}, {"author":"William"}]`)

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

```
$ go test -bench=. -benchmem -benchtime=4s
goos: windows
goarch: amd64
pkg: github.com/bhmj/jsonslice
BenchmarkPath-4        10000000      518 ns/op     336 B/op     8 allocs/op
BenchmarkGet-4          2000000     3761 ns/op     336 B/op     8 allocs/op
BenchmarkUnmarshal-4     300000    22457 ns/op    3568 B/op    88 allocs/op
BenchmarkJsonpath-4     2000000     3927 ns/op     608 B/op    48 allocs/op
PASS
ok      github.com/bhmj/jsonslice       35.874s
```

## Specs

See [jsonpath.com](http://jsonpath.com) for specs and examples

## Limitations and deviations

At the moment a single index reference returns an element, not an array:  
```
main sample0.json $.store.books[0]
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
main sample0.json $.store.books[0:1]
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
main sample1.json $[0].author
```

### Notation

Currently only dot notation (`$.foo.bar`) is supported.

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
  $.obj.length()      -- array lengh or string length, depending on the obj type
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

#### Sub-querying (TODO)
```
  $.obj[any:any].something  -- composite sub-query
  $.obj[3,5,7]              -- multiple array indexes
```
#### Filters (TODO)
```
  @                  -- the current node
  [?(<expression>)]  -- filter expression. Applicable to arrays only
```

### Updates (TODO)

```
  $.obj[?(@.price > 1000)].expensive = true                    -- add/replace field value
  $.obj[?(@.authors.size() > 2)].title += " (multi authored)"  -- expand field value
  $.obj[?(@.price > $.expensive)].bonus = $.bonuses[0].value   -- add/replace field using another jsonpath 
```

## Examples

  Assuming `sample0.json` and `sample1.json` in the example directory:  

  `./main sample0.json '$.store.book[0]'`  
  `./main sample0.json '$.store.book[0].title'`  
  `./main sample0.json '$.store.book[0:-1]'`  
  `./main sample1.json '$[1].author'`  
  `./main sample0.json '$.store.books[?(@.price > 1000)]'`  
  `./main sample0.json '$.store.books[?(@.price > $.expensive)]'`  
  
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
