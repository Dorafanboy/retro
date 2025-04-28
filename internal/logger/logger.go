package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"time"

	"github.com/fatih/color"
)

// Logger defines the interface for application logging.
type Logger interface {
	Info(message string, fields ...interface{})
	InfoWithBlankLine(message string, fields ...interface{})
	Warn(message string, fields ...interface{})
	WarnWithBlankLine(message string, fields ...interface{})
	Error(message string, fields ...interface{})
	ErrorWithBlankLine(message string, fields ...interface{})
	Debug(message string, fields ...interface{})
	DebugWithBlankLine(message string, fields ...interface{})
	Success(message string, fields ...interface{})
	SuccessWithBlankLine(message string, fields ...interface{})
	Highlight(message string, fields ...interface{})
	HighlightWithBlankLine(message string, fields ...interface{})
	Fatal(message string, fields ...interface{})              // Terminates with os.Exit(1)
	FatalWithBlankLine(message string, fields ...interface{}) // Terminates with os.Exit(1)
}

var (
	infoColor      = color.New(color.FgGreen).SprintFunc()
	warnColor      = color.New(color.FgYellow).SprintFunc()
	errorColor     = color.New(color.FgRed).SprintFunc()
	debugColor     = color.New(color.FgCyan).SprintFunc()
	successColor   = color.New(color.FgGreen, color.Bold).SprintFunc()
	highlightColor = color.New(color.FgMagenta).SprintFunc()

	timeColor   = color.New(color.FgWhite).SprintFunc()
	fileColor   = color.New(color.FgBlue).SprintFunc()
	boldColor   = color.New(color.Bold).SprintFunc()
	moduleColor = color.New(color.FgMagenta, color.Bold).SprintFunc()
)

// ColorLogger implements the Logger interface with colored console output.
type ColorLogger struct {
	// Пока нет полей конфигурации
}

// NewColorLogger creates a new instance of ColorLogger.
func NewColorLogger() Logger {
	return &ColorLogger{}
}

// Info logs an informational message.
func (l *ColorLogger) Info(message string, fields ...interface{}) {
	l.printMessage(infoColor("INFO"), message, false, fields...)
}

// InfoWithBlankLine logs an informational message and adds a blank line after it.
func (l *ColorLogger) InfoWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(infoColor("INFO"), message, true, fields...)
}

// Warn logs a warning message.
func (l *ColorLogger) Warn(message string, fields ...interface{}) {
	l.printMessage(warnColor("WARN"), message, false, fields...)
}

// WarnWithBlankLine logs a warning message and adds a blank line after it.
func (l *ColorLogger) WarnWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(warnColor("WARN"), message, true, fields...)
}

// Error logs an error message.
func (l *ColorLogger) Error(message string, fields ...interface{}) {
	l.printMessage(errorColor("ERROR"), message, false, fields...)
}

// ErrorWithBlankLine logs an error message and adds a blank line after it.
func (l *ColorLogger) ErrorWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(errorColor("ERROR"), message, true, fields...)
}

// Debug logs a debug message.
func (l *ColorLogger) Debug(message string, fields ...interface{}) {
	l.printMessage(debugColor("DEBUG"), message, false, fields...)
}

// DebugWithBlankLine logs a debug message and adds a blank line after it.
func (l *ColorLogger) DebugWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(debugColor("DEBUG"), message, true, fields...)
}

// Success logs a success message.
func (l *ColorLogger) Success(message string, fields ...interface{}) {
	l.printMessage(successColor("SUCCESS"), message, false, fields...)
}

// SuccessWithBlankLine logs a success message and adds a blank line after it.
func (l *ColorLogger) SuccessWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(successColor("SUCCESS"), message, true, fields...)
}

// Highlight logs a highlighted message.
func (l *ColorLogger) Highlight(message string, fields ...interface{}) {
	l.printMessage(highlightColor("HIGHLIGHT"), message, false, fields...)
}

// HighlightWithBlankLine logs a highlighted message and adds a blank line after it.
func (l *ColorLogger) HighlightWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(highlightColor("HIGHLIGHT"), message, true, fields...)
}

// Fatal logs a fatal error message and terminates the program via panic.
func (l *ColorLogger) Fatal(message string, fields ...interface{}) {
	l.printMessage(errorColor("FATAL"), message, false, fields...)
	os.Exit(1) // Используем os.Exit(1) вместо panic
}

// FatalWithBlankLine logs a fatal error message, adds a blank line, and terminates.
func (l *ColorLogger) FatalWithBlankLine(message string, fields ...interface{}) {
	l.printMessage(errorColor("FATAL"), message, true, fields...)
	os.Exit(1) // Используем os.Exit(1) вместо panic
}

// printMessage is the internal function for formatting and printing the log message.
func (l *ColorLogger) printMessage(level, message string, addBlankLine bool, fields ...interface{}) {
	formattedPrefix := l.formatMessage(level, message, fields...)
	formattedFields := l.formatFields(fields...)

	fmt.Println(formattedPrefix + formattedFields)

	if addBlankLine {
		fmt.Println()
	}
}

// formatCaller returns information about the call site (file:line)
func (l *ColorLogger) formatCaller() string {
	_, file, line, ok := runtime.Caller(3)
	if !ok {
		return "unknown:0"
	}
	filename := filepath.Base(file)
	return fmt.Sprintf("%s:%d", filename, line)
}

// formatTime returns the current time formatted as YYYY-MM-DD HH:MM:SS
func (l *ColorLogger) formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// extractServiceModule extracts service and module info from key-value fields
func (l *ColorLogger) extractServiceModule(fields ...interface{}) (string, string) {
	var service, module string

	for i := 0; i < len(fields); i += 2 {
		if i+1 >= len(fields) {
			break
		}

		key, ok := fields[i].(string)
		if !ok {
			continue
		}

		value, ok := fields[i+1].(string)
		if !ok {
			continue
		}

		if key == "service" {
			service = value
		} else if key == "module" {
			module = value
		}
	}

	return service, module
}

// formatMessage formats the log message prefix including time, caller, level, and context.
func (l *ColorLogger) formatMessage(level, message string, fields ...interface{}) string {
	service, module := l.extractServiceModule(fields...)

	contextInfo := ""
	if service != "" || module != "" {
		if service != "" && module != "" {
			contextInfo = fmt.Sprintf(" %s[%s:%s]", moduleColor(""), service, module)
		} else if service != "" {
			contextInfo = fmt.Sprintf(" %s[%s]", moduleColor(""), service)
		} else if module != "" {
			contextInfo = fmt.Sprintf(" %s[%s]", moduleColor(""), module)
		}
	}

	baseFormat := "%s %s %s%s %s"

	return fmt.Sprintf(baseFormat,
		timeColor(l.formatTime()),
		fileColor(l.formatCaller()),
		level,
		contextInfo,
		message)
}

// formatField formats a single key-value field for logging.
func (l *ColorLogger) formatField(key string, value interface{}) string {
	if key == "service" || key == "module" {
		return ""
	}
	return fmt.Sprintf("%s=%v", boldColor(key), value)
}

// formatFields formats all additional key-value fields.
func (l *ColorLogger) formatFields(fields ...interface{}) string {
	if len(fields) == 0 {
		return ""
	}

	result := ""
	for i := 0; i < len(fields); i += 2 {
		if i+1 < len(fields) {
			key, ok := fields[i].(string)
			if !ok {
				continue
			}

			if key == "service" || key == "module" {
				continue
			}

			fieldStr := l.formatField(key, fields[i+1])
			if fieldStr != "" {
				result += " " + fieldStr
			}
		}
	}
	return result
}
