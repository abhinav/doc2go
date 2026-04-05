// Package msg defines types for the dependency module.
package msg

// Message is a type from the dependency module.
type Message struct {
	Name string
}

// Container is a generic wrapper from the dependency module.
type Container[T any] struct{ Value T }
