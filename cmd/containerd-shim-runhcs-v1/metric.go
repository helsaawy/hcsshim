//go:build windows

package main

// TODO:
// - initialize metric provider and meter
//  - set to global so HCS code can be instrumented?
// - counters and histograms for svc calls
// - add an interceptor for ttrpc?
// - add auto-counter for HCS/syscall code?
// - add counters (w/ callback) for # GOPROCs, mem size, go routines

// for HCS, see if ttrpc service name can be extracted from context and used as attribute?
