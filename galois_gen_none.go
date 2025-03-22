//go:build !(amd64 || arm64) || noasm || appengine || gccgo || nogen

package reedsolomon

const (
	codeGen              = false
	codeGenMaxGoroutines = 8
	codeGenMaxInputs     = 1
	codeGenMaxOutputs    = 1
	minCodeGenSize       = 1
)
