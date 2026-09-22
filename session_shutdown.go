package main

import "time"

// The timeout is informational, never permission to abandon the writer.
func waitForServerSave(done <-chan bool, reminder time.Duration, notify func()) {
	ticker := time.NewTicker(reminder)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			notify()
		}
	}
}
