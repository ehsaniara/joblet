package logger

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
)

// LogLevel represents the severity level of a log message
type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

func (l LogLevel) String() string {
	switch l {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

type Logger struct {
	level  LogLevel
	logger *log.Logger
	fields map[string]interface{}
	mode   string // to track the mode
}

type Config struct {
	Level  LogLevel
	Output io.Writer
	Format string // "json" or "text" (default)
	Mode   string // "server", "init", or empty
}

func New() *Logger {
	return NewWithConfig(Config{
		Level:  INFO,
		Output: os.Stdout,
		Format: "text",
		Mode:   "", // Default to no mode
	})
}

func NewWithConfig(config Config) *Logger {
	if config.Output == nil {
		config.Output = os.Stdout
	}

	return &Logger{
		level:  config.Level,
		logger: log.New(config.Output, "", 0),
		fields: make(map[string]interface{}),
		mode:   config.Mode,
	}
}

// SetMode sets the mode for the logger (e.g., "server", "init")
func (l *Logger) SetMode(mode string) {
	l.mode = mode
}

// GetMode returns the current mode
func (l *Logger) GetMode() string {
	return l.mode
}

func (l *Logger) WithFields(keyVals ...interface{}) *Logger {
	newLogger := &Logger{
		level:  l.level,
		logger: l.logger,
		fields: make(map[string]interface{}),
		mode:   l.mode, // Preserve mode in new logger
	}

	// copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	for i := 0; i < len(keyVals); i += 2 {
		if i+1 < len(keyVals) {
			key := fmt.Sprintf("%v", keyVals[i])
			newLogger.fields[key] = keyVals[i+1]
		}
	}

	return newLogger
}

// WithField returns a new logger with a single additional context field
func (l *Logger) WithField(key string, value interface{}) *Logger {
	return l.WithFields(key, value)
}

// WithMode returns a new logger with the specified mode
func (l *Logger) WithMode(mode string) *Logger {
	newLogger := &Logger{
		level:  l.level,
		logger: l.logger,
		fields: make(map[string]interface{}),
		mode:   mode,
	}

	// copy existing fields
	for k, v := range l.fields {
		newLogger.fields[k] = v
	}

	return newLogger
}

func (l *Logger) Debug(msg string, keyVals ...interface{}) {
	l.log(DEBUG, msg, keyVals...)
}

func (l *Logger) Info(msg string, kv ...interface{}) {
	l.log(INFO, msg, kv...)
}

func (l *Logger) Warn(msg string, kv ...interface{}) {
	l.log(WARN, msg, kv...)
}

func (l *Logger) Error(msg string, kv ...interface{}) {
	l.log(ERROR, msg, kv...)
}

func (l *Logger) Fatal(msg string, kv ...interface{}) {
	l.log(ERROR, msg, kv...)
	os.Exit(1)
}

func (l *Logger) Fatalf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.log(ERROR, msg)
	os.Exit(1)
}

func (l *Logger) log(level LogLevel, msg string, kv ...interface{}) {
	if level < l.level {
		return
	}

	timestamp := time.Now().Format("2006-01-02T15:04:05.000Z07:00")

	allFields := make(map[string]interface{})
	for k, v := range l.fields {
		allFields[k] = v
	}

	// key/vals from this specific log call
	for i := 0; i < len(kv); i += 2 {
		if i+1 < len(kv) {
			key := fmt.Sprintf("%v", kv[i])
			allFields[key] = kv[i+1]
		}
	}

	logLine := l.formatLogLine(timestamp, level, msg, allFields)

	l.logger.Print(logLine)
}

func (l *Logger) formatLogLine(timestamp string, level LogLevel, msg string, fields map[string]interface{}) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("[%s]", timestamp))
	parts = append(parts, fmt.Sprintf("[%s]", level.String()))

	if l.mode != "" {
		parts = append(parts, fmt.Sprintf("[%s]", l.mode))
	}

	parts = append(parts, msg)

	if len(fields) > 0 {
		var fieldParts []string
		for key, value := range fields {
			fieldParts = append(fieldParts, fmt.Sprintf("%s=%v", key, formatValue(value)))
		}
		if len(fieldParts) > 0 {
			parts = append(parts, fmt.Sprintf("| %s", strings.Join(fieldParts, " ")))
		}
	}

	return strings.Join(parts, " ")
}

func formatValue(value interface{}) string {
	switch v := value.(type) {
	case string:
		// Quote strings that contain spaces
		if strings.Contains(v, " ") {
			return fmt.Sprintf(`"%s"`, v)
		}
		return v
	case error:
		return fmt.Sprintf(`"%s"`, v.Error())
	case time.Duration:
		return v.String()
	case time.Time:
		return v.Format("2006-01-02T15:04:05Z07:00")
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.level = level
}

func (l *Logger) GetLevel() LogLevel {
	return l.level
}

func (l *Logger) IsDebugEnabled() bool {
	return l.level <= DEBUG
}

func (l *Logger) IsInfoEnabled() bool {
	return l.level <= INFO
}

// global logger instance for the convenience
var globalLogger = New()

// SetGlobalMode sets the mode for the global logger
func SetGlobalMode(mode string) {
	globalLogger.SetMode(mode)
}

func Debug(msg string, keyvals ...interface{}) {
	globalLogger.Debug(msg, keyvals...)
}

func Info(msg string, keyvals ...interface{}) {
	globalLogger.Info(msg, keyvals...)
}

func Warn(msg string, keyvals ...interface{}) {
	globalLogger.Warn(msg, keyvals...)
}

func Error(msg string, keyvals ...interface{}) {
	globalLogger.Error(msg, keyvals...)
}

func Fatal(msg string, keyvals ...interface{}) {
	globalLogger.Fatal(msg, keyvals...)
}

func Fatalf(format string, args ...interface{}) {
	globalLogger.Fatalf(format, args...)
}

func WithFields(keyvals ...interface{}) *Logger {
	return globalLogger.WithFields(keyvals...)
}

func WithField(key string, value interface{}) *Logger {
	return globalLogger.WithField(key, value)
}

func WithMode(mode string) *Logger {
	return globalLogger.WithMode(mode)
}

func SetLevel(level LogLevel) {
	globalLogger.SetLevel(level)
}

func ParseLevel(level string) (LogLevel, error) {
	switch strings.ToUpper(level) {
	case "DEBUG":
		return DEBUG, nil
	case "INFO":
		return INFO, nil
	case "WARN", "WARNING":
		return WARN, nil
	case "ERROR":
		return ERROR, nil
	default:
		return INFO, fmt.Errorf("unknown log level: %s", level)
	}
}
