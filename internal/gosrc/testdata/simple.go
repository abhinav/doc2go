package foo

const Constant = 42

var Variable = []byte("foo")

type Struct struct{}

func (*Struct) Method() {}

type Interface interface {
	Method()
}

func Function() {}

type unexportedStruct struct{}
