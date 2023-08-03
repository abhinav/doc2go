// Package highlight provides support to highlight source code blocks.
// It uses the Chroma library to do this work.
//
// Source code blocks are represented as [Code] values,
// which are comprised of multiple [Span]s.
// Spans represent special rendering instructions,
// such as highlighting a region of code,
// or linking to another entity.
package highlight
