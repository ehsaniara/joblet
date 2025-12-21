//go:build linux

package builder

import (
	"fmt"
	"os"
	"time"
)

// BuildLogger provides logging for the build process
type BuildLogger interface {
	Info(format string, args ...interface{})
	Debug(format string, args ...interface{})
	Warn(format string, args ...interface{})
	Error(format string, args ...interface{})
	Phase(phase int, totalPhases int, name string, message string)
}

// DefaultBuildLogger is the default implementation of BuildLogger
type DefaultBuildLogger struct {
	verbose bool
	phase   BuildPhase
}

// NewBuildLogger creates a new build logger
func NewBuildLogger(verbose bool) *DefaultBuildLogger {
	return &DefaultBuildLogger{
		verbose: verbose,
	}
}

func (l *DefaultBuildLogger) timestamp() string {
	return time.Now().Format("15:04:05")
}

func (l *DefaultBuildLogger) Info(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] INFO  %s\n", l.timestamp(), msg)
}

func (l *DefaultBuildLogger) Debug(format string, args ...interface{}) {
	if !l.verbose {
		return
	}
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stdout, "[%s] DEBUG %s\n", l.timestamp(), msg)
}

func (l *DefaultBuildLogger) Warn(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] WARN  %s\n", l.timestamp(), msg)
}

func (l *DefaultBuildLogger) Error(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "[%s] ERROR %s\n", l.timestamp(), msg)
}

func (l *DefaultBuildLogger) Phase(phase int, totalPhases int, name string, message string) {
	l.phase = BuildPhase(phase)
	fmt.Fprintf(os.Stdout, "\n[%s] PHASE %d/%d: %s - %s\n", l.timestamp(), phase, totalPhases, name, message)
}
