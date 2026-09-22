package main

import (
	"testing"
	"time"
)

func TestShutdownWaitDoesNotAbandonSave(t *testing.T) {
	done := make(chan bool)
	returned := make(chan struct{})
	notice := make(chan struct{}, 1)
	go func() {
		waitForServerSave(done, time.Millisecond, func() {
			select {
			case notice <- struct{}{}:
			default:
			}
		})
		close(returned)
	}()
	select {
	case <-notice:
	case <-time.After(time.Second):
		close(done)
		t.Fatal("no slow-save reminder")
	}
	select {
	case <-returned:
		close(done)
		t.Fatal("abandoned save")
	default:
	}
	close(done)
	select {
	case <-returned:
	case <-time.After(time.Second):
		t.Fatal("did not finish")
	}
}
