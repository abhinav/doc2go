// Package gosrc is the part of the pipeline of doc2go
// responsible for finding Go packages and loading information about them.
//
// It provides a [Finder] to search for packages.
// These produce [PackageRef]s, which are references to packages.
// PackageRefs are lightweight and do not contain significant package data,
// so many of them can be in memory at once.
// [Parser] is used to load the actual package data from PackageRefs.
package gosrc
