package main

import (
	"io/ioutil"
)

func mustReadFile(path string) (b []byte) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}
