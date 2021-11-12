package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bhmj/jsonslice"
)

func main() {

	if len(os.Args) < 2 {
		fmt.Printf("Slice out a part of JSON using jsonpath.\nUsage: %[1]s jsonpath <expression> [input_file]\n  ex.1: %[1]s '$.store.book[0].author' sample0.json\n  ex.2: cat sample0.json | %[1]s '$.store.book[0].author'\n", filepath.Base(os.Args[0]))
		return
	}

	var data []byte
	var err error

	if len(os.Args) == 2 {
		data, err = ioutil.ReadAll(os.Stdin)
	} else {
		data, err = ioutil.ReadFile(os.Args[2])
	}
	if err != nil {
		fmt.Println(err)
		return
	}

	s, err := jsonslice.Get(data, os.Args[1])

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(s))
}
