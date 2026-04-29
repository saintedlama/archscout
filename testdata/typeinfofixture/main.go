package main

import (
	"fmt"
	"strings"

	"example.com/typeinfofixture/api"
)

type defaultGreeter struct{}

func (defaultGreeter) Greet(name string) string {
	return "hello " + name
}

func main() {
	// Cross-package qualified function call:
	//   Callee=fmt.Println, CalleeQName="fmt.Println", CalleePackage="fmt"
	fmt.Println("starting")

	// Cross-package qualified function call resolved into the strings pkg:
	//   CalleeQName="strings.ToUpper"
	_ = strings.ToUpper("hello")

	// Cross-package method call via pointer receiver:
	//   CalleeQName="example.com/typeinfofixture/api.Service.Run", IsMethod=true
	svc := &api.Service{}
	_ = svc.Run()

	// Cross-package method call via value receiver:
	//   CalleeQName="example.com/typeinfofixture/api.Service.Describe", IsMethod=true
	val := api.Service{}
	_ = val.Describe()

	// Interface method dispatch:
	//   CalleeQName="example.com/typeinfofixture/api.Greeter.Greet", IsMethod=true
	var g api.Greeter = defaultGreeter{}
	_ = g.Greet("world")

	// Builtin (universe scope): CalleePackage="", CalleeQName="len"
	_ = len("abc")
}
