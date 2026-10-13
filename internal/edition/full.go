//go:build !demo

// Package edition says which build this is: the full game, or the demo
// (built with -tags demo). Demo is a constant, so a branch guarded by it
// compiles away, and code reachable only through one isn't in the binary.
package edition

// Demo is true in the demo build.
const Demo = false
