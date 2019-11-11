
1) comprehensive mode

In comprehensive mode incoming json treated as much versatile as possible to 
match the given jsonpath. Examples: strings that represent integers can be
 treated as such to access array elements by index, dot-notated numeric keys can select array elements as well as object keys, depending on the actual input. This mode tries to emulate JavaScript behaviour. 

path | input 1 | result 1<br>explanation | input 2 | result 2<br>explanation
--- | --- | --- | --- | ---
`$[2]` or `$['2']` or `$.2` or `$.'2'`  | `["a","b","c"]` | `"c"`<br>array element by index |  `{ "1":"a", "2":"b" }` | `"b"`<br>object key by name
`$.*`<br>`$[*]`<br>`$[:]`  | `["a","b","c"]` | `["a","b","c"]`<br>all elements of an array |  `{ "1":"a", "2":"b" }` | `["a", "b"]`<br>values of all the keys
`$[*].bar`<br>`$.*.bar` | `[{"foo":1},{"bar":2}]` | `[2]`<br>find a key in every element |  `{"a":{"foo":1},"b":{"bar":2}}` | `[2]`<br>find a key in every value
`$[1,2]`<br>`$['1','2']` | `["a","b","c"]` | `["b","c"]`<br>aggregate array elements by index |  `{"1":"a", "2":"b"}}` | `["a","b"]`<br>aggregate values of keys by name

2) strict mode

This mode is useful when you need to be thorough about the input data. In strict mode dot notation is used solely for accessing object fields by name thus allowing to distinguish between objects and arrays in ambiguous cases, while bracket notation is used for:  
a) indexing or aggregating array elements -- in this case all values inside brackets must be numeric  
b) aggregating key values in objects -- in this case all values inside brackets must be alphanumeric or quoted strings.  
c) selecting a single key value from object -- this does not imply aggregation and treated as a synonym for dot notation (for compatibility reasons).  
Similarly, dot-notated wildcard is only applicable to objects.

path | input 1 | result 1<br>explanation | input 2 | result 2<br>explanation
--- | --- | --- | --- | ---
`$[2]`  | `["a","b","c"]` | `"c"`<br>array element by index |  `{ "1":"a", "2":"b" }` | <span style="color:#DD4444">not applicable:<br>trying to index an object</span>
`$.2` or `$.'2'` | `["a","b","c"]` | <span style="color:#DD4444">dot notation is not applicable for array</span> |  `{ "1":"a", "2":"b" }` | `"b"`<br>object key by name
`$['2']` | `["a","b","c"]` | <span style="color:#DD4444">not applicable:<br>non-integer index</span> |  `{ "1":"a", "2":"b" }` | `"b"`<br>object key by name
`$['1','2']` | `["a","b","c"]` | <span style="color:#DD4444">not applicable:<br>non-integer indexes</span> |  `{"1":"a", "2":"b"}}` | `["a","b"]`<br>aggregate values of keys by name
`$[1,2]` | `["a","b","c"]` | `["b","c"]`<br>aggregate array elements by index |  `{"1":"a", "2":"b"}}` | `["a","b"]`<br>aggregate values of keys by name
`$[*]`  | `["a","b","c"]` | `["a","b","c"]`<br>all elements of an array |  `{ "1":"a", "2":"b" }` | `["a", "b"]`<br>values of all the keys (aggregation)
`$.*`  | `["a","b","c"]` | <span style="color:#DD4444">dot notation is not applicable for array</span> |  `{ "1":"a", "2":"b" }` | `["a", "b"]`<br>values of all the keys (aggregation)

### Schema

```jsonpath:
	${ref}{ref}...
	@{ref}{ref}...
ref:
	.|..{keyref}
	|.|..|{brackets}
keyref:
	key|brackets
brackets:
	[{someth}]
key:
	string
	word
	index
	*
someth:
	?({expr})
	{key}
	{key},{key}...
	{index},{index}...
	{start}:{end}
	{start}:{end}:{step}
index, start, end, step:
	integer
expr:
	{operand} {operator}{operand}
operand:
	jsonpath
	value
value:
	string
	integer
	float
	'null'
	regexp
operator:
	==,!=,>,<,>=,<=,&&,||
string:
	"..."
	'...'
	`...`
```

ref types:
example | type | applicable to | flags | notes (NF = not found)
--- | --- | --- | --- | ---
`$.key` or `$.'key'` | single word key | object | **common** | NF on arrays
`$.3` or `$[3]` or `$['3']` or `$.[3]` or `$.['3']` | single numeric key == index | object or array | **common**
`$[1,2]` | union | object or array | **aggregating** | 
`$[1,'a']` | union | object or array | **aggregating** | word keys NF on arrays
`$[1,'a']` | union | object or array | **aggregating** | word keys NF on arrays
`$['*']` | == `$.'*'` | object | **common** | syntax to get a value of a `*` key
`$.key.size()` | function | object or array | **function** | 
`$[xx:yy:zz]` | slice | array | **slice** | NF on objects
`$[:]` | slice | array | **slice** | == `$.*` in comprehensive mode (?)
`$..key` or `$..['key']` | sigle word key | object or array | **deepscan** | 
`$..[0]` or `$..['0']` | sigle word key == index | object or array | **deepscan** | 
`$..[0:2]` |  | array | **deepscan** | 
`$.*` or `$[*]` | wildcard | object or array | **wildcard**
`$..*` or `$..[*]` | deepscan wildcard | object or array | **deepscan** **wildcard**

- common ($.book or $[0]) -- cDot
	- array
		- word ref: NF
		- index: by index
	- object
		- word ref: by name
		- index: NF
	- base type
		- NF
- aggregating ($[1,2] or $['a','b']) -- cAgg
	- array
		- word ref: NF
		- index: AGG( by index )
	- object
		- word ref: AGG( by name )
		- index: NF
- slice ($[0:3]) -- cSlice
	- array 
		- AGG( slice elems )
	- object
		- NF
- function -- cFunction
	- array
		- length or size
	- string
		- string length
- wildcard (.* or [:] or [*]) -- cWild
	- array
		- AGG( all elements )
	- object
		- AGG( values of all keys )
- deepscan ($..book or $..[0] or $..['foo','bar']) -- cDeep
	- array
		- word ref: AGG( recurse on every elem )
		- index: AGG( get elem by index + recurse on every elem )
	- object
		- word ref: AGG( by name + recurse on every kvalue )
		- index: AGG( recurse on every kvalue )
- deepscan wildcard ($..* or $..[*] or $..[:])
	- array
		- AGG( all elems + recurse on every elem )
	- object
		- word ref: AGG( all kvalues + recurse on every kvalue )

ref flags:

	- common		- common node
	- terminal		- no more refs follow, return result
	- union
		- object: collect values of keys, return array
		- array: collect specified elems, return array
	- function		- apply function to the last value
	- slice			- slice elems (the subject must be array)
	- filter		- apply filter (the subject must be array)
	- wildcard		- wildcard for object or array. Result is array.
	- deepscan		- deepscan. Result is array

array modes:

	single
		[1]     - counting scan 
		[-1]    - fullscan & cut
	
	multi
		[1,2]   - counting scan 
		[-1,2]  - fullscan & cut
	
	slice
		[:]     - [cEmpty:cEmpty] fullscan & cut
		[2:]    - [INT:cEmpty]    fullscan & cut
		[:5]    - [cEmpty:INT]    fullscan & cut
		[-2:]   - [-INT:cEmpty]   fullscan & cut
		[:-5]   - [cEmpty:-INT]   fullscan & cut
		[2:5]   - [INT:INT]       counting scan
		[-5:2]  - [-INT:INT]      fullscan & cut
		[1:-3]  - [INT:-INT]      fullscan & cut
		[-5:-3] - [-INT:-INT]     fullscan & cut

	slice with step
		fullscan & cut
