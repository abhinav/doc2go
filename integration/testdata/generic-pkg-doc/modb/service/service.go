// Package service uses dependency types
// both directly and as generic type arguments.
package service

import msga "example.com/moda/msg"

// UsesGenericArg demonstrates a cross-module type reference
// inside a generic type argument.
func UsesGenericArg(_ msga.Container[msga.Message]) {}

// UsesDirectRef demonstrates a plain cross-module type reference.
func UsesDirectRef(_ *msga.Message) {}
