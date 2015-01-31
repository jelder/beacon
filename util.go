package main

import (
	"io/ioutil"
)

func variadicScriptArgs(args []string) []interface{} {
	scriptArgs := make([]interface{}, len(args)+1)
	scriptArgs[0] = len(args) // First argument is length
	for i, v := range args {
		scriptArgs[i+1] = interface{}(v)
	}
	return scriptArgs
}

func mustReadFile(path string) (b []byte) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return b
}
