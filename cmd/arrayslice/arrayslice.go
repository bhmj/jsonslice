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
		fmt.Printf("Slice out array elements from JSON.\nUsage: %[1]s jsonpath [input_file]\n  ex.1: %[1]s '$.store.book[1:3]' sample0.json\n  ex.2: cat sample0.json | %[1]s '$.store.book[1:3]'\n", filepath.Base(os.Args[0]))
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

	s, err := jsonslice.GetArrayElements(data, os.Args[1], 20)

	if err != nil {
		fmt.Println(err)
		return
	}

	for i := 0; i < len(s); i++ {
		fmt.Println(string(s[i]))
	}
}
