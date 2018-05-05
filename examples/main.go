package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/bhmj/jsonslice"
)

func main() {

	if len(os.Args) < 3 {
		fmt.Printf("Usage: %[1]s <input_file> <jsonpath>\n  ex: %[1]s sample0.json $.store.book[0].author", filepath.Base(os.Args[0]))
		return
	}

	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		fmt.Println(err)
		return
	}

	s, err := jsonslice.Get(data, os.Args[2])

	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println(string(s))
}
