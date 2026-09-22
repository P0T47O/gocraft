//go:build !windows

package main

import "fmt"

func reportNativeGraphicsFailure(err error) {
	fmt.Println(graphicsFailureMessage(err, "graphics-error.log"))
}
