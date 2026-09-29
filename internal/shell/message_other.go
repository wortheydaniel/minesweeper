//go:build !windows

package shell

import (
	"fmt"
	"os"
)

// Message prints text for the user: errors to stderr, information to stdout.
func Message(title, text string, isError bool) {
	if isError {
		fmt.Fprintln(os.Stderr, text)
		return
	}
	fmt.Fprintln(os.Stdout, text)
}
