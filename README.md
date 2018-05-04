# JSON Slice

## Changelog

2018-05-04 - definite paths supported (array slice, object or single value)  
2018-05-01 - first commit

## Getting started

JsonSlice is a Go package which allows to execute fast jsonpath queries without unmarshalling the whole data.

## Specs

### notation

Currently only dot notation (`$.foo.bar`) is supported.

### operators
```
    $                   -- root node (can be either object or array)
    @                   -- (TODO) the current node (in a filter)
    .node               -- dot-notated child
    [123]               -- array index
    [12:34]             -- array bound
    [?(<expression>)]   -- (TODO) filter expression. Applicable to arrays only.
```
### functions (TODO)
```
  $.obj.length() -- array lengh or string length, depending on the obj type
  $.obj.size() -- object size in bytes (as is)
```
## definite
```
  $.obj
  $.obj.val
  // arrays: indexed
  $.obj[3]
  $.obj[3].val
  $.obj[-2]  -- second from the end
```
### indefininte
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
### filters (TODO)
```
  $.obj[?(@.price > 1000)] -- filter expression
```

## Examples

  $.obj[?(@.price > $.average)]
  $[0].compo[1].name

## Contributing
1. Fork it!
2. Create your feature branch: `git checkout -b my-new-feature`
3. Commit your changes: `git commit -am 'Add some feature'`
4. Push to the branch: `git push origin my-new-feature`
5. Submit a pull request :)

## Limitations

## Licence

[MIT](http://opensource.org/licenses/MIT)

## Author

Michael Gurov aka BHMJ
