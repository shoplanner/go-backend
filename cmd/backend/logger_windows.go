//go:build windows

package main

import (
	"io"
	"os"
)

// newLogWriter logs to stderr on Windows, where the syslog package is
// unavailable. The LOG_NO_SYSLOG switch is a no-op here.
func newLogWriter() io.Writer {
	return os.Stderr
}
