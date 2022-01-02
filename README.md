[![Build Status](https://travis-ci.org/bhmj/jsonslice.svg?branch=master)](https://travis-ci.org/bhmj/jsonslice)
[![Go Report Card](https://goreportcard.com/badge/github.com/bhmj/jsonslice)](https://goreportcard.com/report/github.com/bhmj/jsonslice)
[![GoDoc](https://godoc.org/github.com/bhmj/jsonslice?status.svg)](https://godoc.org/github.com/bhmj/jsonslice)

# JSON Slice

## What is it?

JSON Slice is a Go package which allows to execute fast jsonpath queries without unmarshalling the whole data.  

Sometimes you need to get a single value from incoming json using jsonpath, for example to route data accordingly or so. To do that you must unmarshall the whole data into interface{} struct and then apply some jsonpath library to it, only to get just a tiny little value. What a waste of resourses! Well, now there's `jsonslice`.

Simply call `jsonslice.Get` on your raw json data to slice out just the part you need. The `[]byte` received can then be unmarshalled into a struct or used as it is.

## Getting started

#### 1. install

```
$ go get github.com/bhmj/jsonslice
```

#### 2. use it

```golang
import "github.com/bhmj/jsonslice"
import "fmt"

func main() {
    var data = []byte(`
    { "sku": [ 
        { "id": 1, "name": "Bicycle", "price": 160, "extras": [ "flashlight", "pump" ] },
        { "id": 2, "name": "Scooter", "price": 280, "extras": [ "helmet", "gloves", "spare wheel" ] }
      ]
    } `)

    a, _ := jsonslice.Get(data, "$.sku[0].price")
    b, _ := jsonslice.Get(data, "$.sku[1].extras.count()")
    c, _ := jsonslice.Get(data, "$.sku[?(@.price > 200)].name")
    d, _ := jsonslice.Get(data, "$.sku[?(@.extras.count() < 3)].name")

    fmt.Println(string(a)) // 160
    fmt.Println(string(b)) // 3
    fmt.Println(string(c)) // ["Scooter"]
    fmt.Println(string(d)) // ["Bicycle"]
}
```
[Run in Go Playground](https://play.golang.org/p/fYv-Y12akvs)

## Package functions
  
`jsonslice.Get(data []byte, jsonpath string) ([]byte, error)`  
  - get a slice from raw json data specified by jsonpath

## Specs and references

* Originally based on [Stefan Gössner's article](http://goessner.net/articles/JsonPath/index.html#e2).
* Expression evaluator designed to comply with [ECMAScript Language: Expressions reference](https://tc39.es/ecma262/multipage/ecmascript-language-expressions.html).
* A proud member of amazing [Christoph Burgmer's json-path-comparison](https://github.com/cburgmer/json-path-comparison).

## Syntax features

1. Classic dot notation (`$.simple_key`) is limited to alphanumeric characters. For more complex cases use `$['complex key!']` or `$.'complex key!'`. 

2. A single index reference returns an element, not an array; a slice reference always returns array:  
```
$ echo '[{"name":"abc"}, {"name":"def"}]' | ./build/jsonslice '$[0].name' 
"abc"
```
```
$ echo '[{"name":"abc"}, {"name":"def"}]' | .build//jsonslice '$[0:1].name'
["abc"]
```

3. Indexing or slicing on root node is supported (assuming json is an array and not an object):  
```
$ cat example/sample1.json | ./build/jsonslice '$[0].author'
```

## Expressions

### Overview 
```
  $                   -- root node (can be either object or array)
  $.node              -- dot-notated child
  $.node.deeper       -- dot-notated child (chain of them)
  $.'some node'       -- dot-notated child (syntax extension)
  ['node']            -- bracket-notated child
  ['foo','bar']       -- bracket-notated children (aggregation)
  [5]                 -- array index
  [-5]                -- negative index means "from the end"
  [1:9]               -- array slice
  [1:9:2]             -- array slice (+step)
  .*  .[*]  .[:]      -- wildcard
  ..key               -- deepscan
  .'\''               -- escape sequences supported (\", \', \n, \r, \t, \0, \\)
  .'k\x0A'            -- escaped hex bytes supported
  .'k\u00F6'          -- escaped 16-bit unicode codepoints supported
  .'k\U000000F6'      -- escaped 32-bit unicode codepoints supported
```
### Functions
```
  $.obj.length()      -- number of elements in an array or string length, depending on the obj type
  $.obj.count()       -- same as above
  $.val.size()        -- value size in bytes (as is)
```
### Slices
```
  $.arr[start:end:step]
  $.arr[start:end]
```
Selects elements from `start` (inclusive) to `end` (exclusive), stepping by `step`. If `step` is omitted or zero, then 1 is implied. Out-of-bounds values are reduced to the nearest bound.

If `step` is positive:
  - empty `start` treated as the first element inclusive
  - empty `end` treated as the last element inclusive
  - `start` should be less then `end`, otherwise result will be empty

If `step` is negative:
  - empty `start` treated as last element inclusive
  - empty `end` treated as the first element inclusive
  - `start` should be greater then `end`, otherwise result will be empty

### Filters

```
  [?(<expression>)]  -- filter expression. Applicable to arrays only
  @                  -- the root of the current element of the array. Used only within a filter.
  @.val              -- a field of the current element of the array.
```

#### Filter operators

  Operator | Description
  --- | ---
  math  | `+` `-` `*` `/` `%` `**`
  `===`  | Strict equality (mimics JavaScript). Examples: `true===true, 42===42`
  `==`  | Abstract equality (mimics JavaScript). Examples: `true=="1", 42=="42"`. <br>Use single or double quotes for string expressions.<br>`[?(@.color=='red')]` or `[?(@.color=="red")]`
  `!=`  | Abstract not equal to<br>`[?(@.author != "Herman Melville")]`
  `!==`  | Strict not equal to<br>`?(@.tag !== "1")`
  `>`   | Greater than<br>`[?(@.price > 10)]`
  `>=`  | Greater than or equal to
  `<`   | Less than
  `<=`  | Less than or equal to
  `=~`  | Match a regexp<br>`[?(@.name =~ /sword.*/i]`
  `!~` or `!=~`  | Don't match a regexp<br>`[?(@.name !~ /sword.*/i]`
  `&&`  | Logical AND<br>`[?(@.price < 10 && @isbn)]`
  `\|\|`  | Logical OR<br>`[?(@.price > 10 \|\| @.category == 'reference')]`
  `!`  | Logical NOT<br>`[?(!@.is_expensive)]`
  `\|`  | Bitwise OR<br>`[?(@.bits\|@.pieces > 0)]`
  `&`  | Bitwise AND<br>`[?(@.bits & 7 == 1)]`
  `^`  | Bitwise XOR<br>`[?(@.bits ^ 1 > 0)]`
  `~`  | Bitwise NOT<br>`[?(~@.bits == 0xF0)]`
  `<<`  | Bitwise left shift<br>`[?(@.bits << 1 == 2)]`
  `>>`  | Bitwise right shift<br>`[?(@.bits >> 1 == 0)]`

#### Comparison details
Comparison mostly complies with JavaScript specifications, see [Testing and Comparison Operations](https://tc39.es/ecma262/multipage/abstract-operations.html#sec-testing-and-comparison-operations).   
If you encounter wrong or inconsistent comparison behaviour please let me know by creating an issue in this repository.

## Examples

Assuming `sample0.json` and `sample1.json` in the example directory:  

  `cat example/sample0.json | ./build/jsonslice '$.store.book[0]'`  
  `cat example/sample0.json | ./build/jsonslice '$.store.book[0].title'`  
  `cat example/sample0.json | ./build/jsonslice '$.store.book[0:-1]'`  
  `cat example/sample1.json | ./build/jsonslice '$[1].author'`  
  `cat example/sample0.json | ./build/jsonslice '$.store.book[?(@.price > 10)]'`  
  `cat example/sample0.json | ./build/jsonslice '$.store.book[?(@.price > $.expensive)]'`  

Much more examples can be found in `jsonslice_test.go`  

## Benchmarks (Core i9-9880H)

```diff
$ go test -bench=. -benchmem -benchtime=4s
goos: darwin
goarch: amd64
pkg: github.com/bhmj/jsonslice
cpu: Intel(R) Core(TM) i9-9880H CPU @ 2.30GHz
++ usually you need to unmarshal the whole JSON to get an object by jsonpath (for reference):
Benchmark_Unmarshal-16                    398998      11268 ns/op    4272 B/op    130 allocs/op
++ and here's a jsonslice.Get:
Benchmark_Jsonslice_Get_Simple-16        1660604       2885 ns/op      24 B/op      1 allocs/op
++ Get() involves parsing a jsonpath, here it is:
Benchmark_JsonSlice_ParsePath-16        11955015        400 ns/op       0 B/op      0 allocs/op
++ if you aggregate some non-contiguous elements, it may take a bit longer (extra mallocs involved):
Benchmark_Jsonslice_Get_Aggregated-16    1000000       4335 ns/op     313 B/op     10 allocs/op
++ usual unmarshalling a large json:
Benchmark_Unmarshal_10Mb-16                  100   40787414 ns/op     224 B/op      5 allocs/op
++ jsonslicing the same json, target element is near the start:
Benchmark_Jsonslice_Get_10Mb_First-16    3459492       1370 ns/op      24 B/op      1 allocs/op
++ jsonslicing the same json, target element is near the end: still beats Unmarshal
Benchmark_Jsonslice_Get_10Mb_Last-16         133   35731931 ns/op      54 B/op      1 allocs/op
PASS
ok      github.com/bhmj/jsonslice       52.452s
```

## Changelog

**1.1.2** (2022-01-02) -- Unicode support added. Expression parser upgrade to v0.9.1  
Bugfix: indexing of array element inside expression (`@[-1]`).  
Bugfix: ecaped backslash in node key now works (`["\\"]`).  
See `Test_Fixes` function for bugfix details.

**1.1.1** (2022-10-20) -- Expression parser upgrade to v0.9.0

**1.1.0** (2021-11-12) -- Expression parser/evaluator has been separated to [different project](https://github.com/bhmj/xpression/) and completely rewritten. Parentheses now fully supported. Exponentiation operator added (`**`). Bitwise operators added (`|`, `&`, `^`, `<<`, `>>`). All expression calculations are JavaScript-compliant.

**1.0.6** (2021-10-31) -- JS-like behaviour on string/number/boolean values comparison. `===` operator added for strict comparison. Strings are now comparable.

**1.0.5** (2020-09-22) -- bugfix: `$..many.keys` used to trigger on `many` without recursing deeper on `keys`.

**1.0.4** (2020-05-07) -- bugfix: `$*` path generated panic.

**1.0.3** (2019-12-24) -- bugfix: `$[0].foo` `[{"foo":"\\"}]` generated "unexpected end of input".

**1.0.2** (2019-12-07) -- nested aggregation (`$[:].['a','b']`) now works as expected. TODO: add option to switch nested aggregation mode at runtime!

**1.0.1** (2019-12-01) -- "not equal" regexp operator added (`!=~` or `!~`).

**1.0.0** (2019-11-29) -- deepscan operator (`..`) added, slice with step `$[1:9:2]` is now supported, syntax extensions added. `GetArrayElements()` removed.

**0.7.6** (2019-09-11) -- bugfix: escaped backslash at the end of a string value.

**0.7.5** (2019-05-21) -- Functions `count()`, `size()`, `length()` work in filters.
> `$.store.bicycle.equipment[?(@.count() == 2)]` -> `[["light saber", "apparel"]]`  

**0.7.4** (2019-03-01) -- Mallocs reduced (see Benchmarks section).

**0.7.3** (2019-02-27) -- `GetArrayElements()` added.

**0.7.2** (2018-12-25) -- bugfix: closing square bracket inside a string value.

**0.7.1** (2018-10-16) -- bracket notation is now supported.
> `$.store.book[:]['price','title']` -> `[[8.95,"Sayings of the Century"],[12.99,"Sword of Honour"],[8.99,"Moby Dick"],[22.99,"The Lord of the Rings"]]`

**0.7.0** (2018-07-23) -- Wildcard key (`*`) added.
> `$.store.book[-1].*` -> `["fiction","J. R. R. Tolkien","The Lord of the Rings","0-395-19395-8",22.99]`  
> `$.store.*[:].price` -> `[8.95,12.99,8.99,22.99]`

**0.6.3** (2018-07-16) -- Boolean/null value error fixed.

**0.6.2** (2018-07-03) -- More tests added, error handling clarified.

**0.6.1** (2018-06-26) -- Nested array indexing is now supported.
> `$.store.bicycle.equipment[1][0]` -> `"peg leg"`

**0.6.0** (2018-06-25) -- Regular expressions added.
> `$.store.book[?(@.title =~ /(dick)|(lord)/i)].title` -> `["Moby Dick","The Lord of the Rings"]`

**0.5.1** (2018-06-15) -- Logical expressions added.
> `$.store.book[?(@.price > $.expensive && @.isbn)].title` -> `["The Lord of the Rings"]`

**0.5.0** (2018-06-14) -- Expressions (aka filters) added.
> `$.store.book[?(@.price > $.expensive)].title` -> `["Sword of Honour","The Lord of the Rings"]`

**0.4.0** (2018-05-16) -- Aggregating sub-queries added.
> `$.store.book[1:3].author` -> `["John","William"]`

**0.3.0** (2018-05-05) -- MVP.

## Roadmap

- [x] length(), count(), size() functions
- [x] filters: simple expressions
- [x] filters: complex expressions (with logical operators)
- [x] nested arrays support
- [x] wildcard operator (`*`)
- [x] bracket notation for multiple field queries (`$['a','b']`)
- [x] deepscan operator (`..`)
- [x] syntax extensions: `$.'keys with spaces'.price`
- [x] flexible syntax: `$[0]` works on both `[1,2,3]` and `{"0":"abc"}`
- [x] JavaScript-compatible expressions
- [x] Unicode support
- [ ] IN (), NOT IN ()
- [ ] cache parsed jsonpaths of used variables at filterMatch.varFunc(str)
- [ ] Optionally unmarshal the result
- [ ] Option to select aggregation mode (nested or plain)

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
