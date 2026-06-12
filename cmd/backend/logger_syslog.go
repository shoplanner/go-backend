//go:build !windows

package main

import (
	"io"
	"log/syslog"
	"os"

	"github.com/rs/zerolog"
)

// newLogWriter returns the destination for application logs.
//
// By default logs are sent to syslog. Set LOG_NO_SYSLOG to any non-empty value
// (handy for local runs without a syslog daemon) to log to stderr instead.
// If syslog is requested but unavailable, we fall back to stderr rather than
// crashing the process.
func newLogWriter() io.Writer {
	if os.Getenv("LOG_NO_SYSLOG") != "" {
		return os.Stderr
	}

	writer, err := syslog.New(syslog.LOG_DEBUG, os.Args[0])
	if err != nil {
		return os.Stderr
	}

	return zerolog.SyslogLevelWriter(writer)
}
