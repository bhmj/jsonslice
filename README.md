# JSON Slice

## Changelog

2018-05-05 - array slice works, bugfixes  
2018-05-04 - definite paths supported (array slice, object or single value)  
2018-05-01 - first commit

## Getting started

JsonSlice is a Go package which allows to execute fast jsonpath queries without unmarshalling the whole data.

## Benchmarks

```
$ cd examples/ && go test -bench=. -benchmem -benchtime=3s && cd ..
goos: windows
goarch: amd64
pkg: github.com/bhmj/jsonslice/examples
BenchmarkGet-4           1000000              4294 ns/op             368 B/op         16 allocs/op
BenchmarkJsonpath-4       200000             28371 ns/op            4192 B/op        137 allocs/op
PASS
ok      github.com/bhmj/jsonslice/examples      10.348s
```

## Specs

See [jsonpath.com](jsonpath.com) for specs and examples

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

Also, indexing on root node is supported (assuming json is an array and not an object):  
```
main sample1.json $[0].author
```

### Notation

Currently only dot notation (`$.foo.bar`) is supported.

### Operators
```
    $                   -- root node (can be either object or array)
    @                   -- (TODO) the current node (in a filter)
    .node               -- dot-notated child
    [123]               -- array index
    [12:34]             -- array bound
    [?(<expression>)]   -- (TODO) filter expression. Applicable to arrays only.
```
### Functions (TODO)
```
  $.obj.length() -- array lengh or string length, depending on the obj type
  $.obj.size() -- object size in bytes (as is)
```
### Definite
```
  $.obj
  $.obj.val
  // arrays: indexed
  $.obj[3]
  $.obj[3].val
  $.obj[-2]  -- second from the end
```
### Indefininte
```
  // arrays: bounded
  $.obj[:]   -- == $.obj (all elements of the array)
  $.obj[0:]  -- the same as above: items from index 0 (inclusive) till the end
  $.obj[<anything>:0] -- doesn't make sence (from some element to the index 0 exclusive -- which is always empty)
  $.obj[2:]  -- items from index 2 (inclusive) till the end
  $.obj[:5]  -- items from the beginning to the index 5 (exclusive)
  $.obj[-2:] -- items from the second element from the end (inclusive) till the end
  $.obj[:-2] -- items from the beginning to the second element from the end (exclusive)
  $.obj[3:5] -- items from index 2 (inclusive) to the index 5 (exclusive)
```
### sub-querying (TODO)
```
  $.obj[any:any].something -- composite sub-query
  $.obj[3,5,7] -- multiple array indexes
```
### Filters (TODO)
```
  $.obj[?(@.price > 1000)] -- filter expression
```

## Examples

  Considering `sample0.json` and `sample1.json` in the example directory:  

  `./main sample0.json '$.store.book[0]'`  
  `./main sample0.json '$.store.book[0].title'`  
  `./main sample0.json '$.store.book[0:-1]'`  
  `./main sample1.json '$[1].author'`  
  
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