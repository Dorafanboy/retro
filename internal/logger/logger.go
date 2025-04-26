package logger

import (
	"fmt"
	"github.com/fatih/color"
	"path/filepath"
	"runtime"
	"time"
)

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

// Info logs an informational message.
func Info(message string, fields ...interface{}) {
	printMessage(infoColor("INFO"), message, false, fields...)
}

// InfoWithBlankLine logs an informational message and adds a blank line after it.
func InfoWithBlankLine(message string, fields ...interface{}) {
	printMessage(infoColor("INFO"), message, true, fields...)
}

// Warn logs a warning message.
func Warn(message string, fields ...interface{}) {
	printMessage(warnColor("WARN"), message, false, fields...)
}

// WarnWithBlankLine logs a warning message and adds a blank line after it.
func WarnWithBlankLine(message string, fields ...interface{}) {
	printMessage(warnColor("WARN"), message, true, fields...)
}

// Error logs an error message.
func Error(message string, fields ...interface{}) {
	printMessage(errorColor("ERROR"), message, false, fields...)
}

// ErrorWithBlankLine logs an error message and adds a blank line after it.
func ErrorWithBlankLine(message string, fields ...interface{}) {
	printMessage(errorColor("ERROR"), message, true, fields...)
}

// Debug logs a debug message.
func Debug(message string, fields ...interface{}) {
	printMessage(debugColor("DEBUG"), message, false, fields...)
}

// DebugWithBlankLine logs a debug message and adds a blank line after it.
func DebugWithBlankLine(message string, fields ...interface{}) {
	printMessage(debugColor("DEBUG"), message, true, fields...)
}

// Success logs a success message.
func Success(message string, fields ...interface{}) {
	printMessage(successColor("SUCCESS"), message, false, fields...)
}

// SuccessWithBlankLine logs a success message and adds a blank line after it.
func SuccessWithBlankLine(message string, fields ...interface{}) {
	printMessage(successColor("SUCCESS"), message, true, fields...)
}

// Highlight logs a highlighted message.
func Highlight(message string, fields ...interface{}) {
	printMessage(highlightColor("HIGHLIGHT"), message, false, fields...)
}

// HighlightWithBlankLine logs a highlighted message and adds a blank line after it.
func HighlightWithBlankLine(message string, fields ...interface{}) {
	printMessage(highlightColor("HIGHLIGHT"), message, true, fields...)
}

// Fatal logs a fatal error message and terminates the program via panic.
func Fatal(message string, fields ...interface{}) {
	printMessage(errorColor("FATAL"), message, false, fields...)
	panic(message) // Panic is used to stop execution immediately
}

// FatalWithBlankLine logs a fatal error message, adds a blank line, and terminates.
func FatalWithBlankLine(message string, fields ...interface{}) {
	printMessage(errorColor("FATAL"), message, true, fields...)
	panic(message)
}

// printMessage is the internal function for formatting and printing the log message.
func printMessage(level, message string, addBlankLine bool, fields ...interface{}) {
	formattedPrefix := formatMessage(level, message, fields...)
	formattedFields := formatFields(fields...)

	fmt.Println(formattedPrefix + formattedFields)

	if addBlankLine {
		fmt.Println()
	}
}

// formatCaller returns information about the call site (file:line)
func formatCaller() string {
	_, file, line, ok := runtime.Caller(2)
	if !ok {
		return "unknown:0"
	}
	filename := filepath.Base(file)
	return fmt.Sprintf("%s:%d", filename, line)
}

// formatTime returns the current time formatted as YYYY-MM-DD HH:MM:SS
func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

// extractServiceModule extracts service and module info from key-value fields
func extractServiceModule(fields ...interface{}) (string, string) {
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
func formatMessage(level, message string, fields ...interface{}) string {
	service, module := extractServiceModule(fields...)

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
		timeColor(formatTime()),
		fileColor(formatCaller()),
		level,
		contextInfo,
		message)
}

// formatField formats a single key-value field for logging.
// Skips service and module fields as they are handled in the prefix.
func formatField(key string, value interface{}) string {
	if key == "service" || key == "module" {
		return ""
	}
	return fmt.Sprintf("%s=%v", boldColor(key), value)
}

// formatFields formats all additional key-value fields.
func formatFields(fields ...interface{}) string {
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

			fieldStr := formatField(key, fields[i+1])
			if fieldStr != "" {
				result += " " + fieldStr
			}
		}
	}
	return result
}
