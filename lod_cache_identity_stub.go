//go:build !windows

package main

func lodLocalCacheIdentity(path string) string { return path }
