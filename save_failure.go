package main

import (
	"fmt"
	"os"
	"time"
)

func saveFailureMessage(cause error, path string) string {
	message := fmt.Sprintf("World save failed: %v", cause)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err == nil {
		_, err = fmt.Fprintf(f, "%s %s\n", time.Now().Format(time.RFC3339), message)
		closeErr := f.Close()
		if err == nil {
			err = closeErr
		}
	}
	if err != nil {
		return fmt.Sprintf("%s; cannot write save error log: %v", message, err)
	}
	return message + "; details: " + path
}
