package utils

import (
	"fmt"
	"log"
	"os"
)

type Logger struct {
	logger *log.Logger
}

func NewLogger() (*Logger, error) {
	logger := log.New(os.Stdout, "", log.LstdFlags)

	return &Logger{
		logger: logger,
	}, nil
}

func (l *Logger) Info(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[INFO] %s", message)
}

func (l *Logger) Error(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[ERROR] %s", message)
}

func (l *Logger) Warning(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[WARNING] %s", message)
}

func (l *Logger) Debug(format string, args ...interface{}) {
	message := fmt.Sprintf(format, args...)
	l.logger.Printf("[DEBUG] %s", message)
}

func (l *Logger) Close() error {
	return nil
}