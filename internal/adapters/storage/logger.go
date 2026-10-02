package storage

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel define el nivel de criticidad del log
type LogLevel string

const (
	LevelDebug LogLevel = "DEBUG"
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

// FileLogger gestiona el volcado diario de trazas en ~/.sdd/logs/sdd-YYYYMMDD.log
type FileLogger struct {
	mu         sync.Mutex
	logDir     string
	debugMode  bool
	verbose    bool
	currentDay string
	file       *os.File
}

var globalLogger *FileLogger
var once sync.Once

// InitGlobalLogger inicializa el logger central del CLI
func InitGlobalLogger(debug, verbose bool) (*FileLogger, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("error obteniendo home: %w", err)
	}
	logDir := filepath.Join(home, ".specforge", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("error creando directorio de logs %s: %w", logDir, err)
	}

	logger := &FileLogger{
		logDir:    logDir,
		debugMode: debug,
		verbose:   verbose,
	}

	once.Do(func() {
		globalLogger = logger
	})

	return logger, nil
}

// GetLogger devuelve la instancia global o una instancia en memoria
func GetLogger() *FileLogger {
	if globalLogger == nil {
		home, _ := os.UserHomeDir()
		logDir := filepath.Join(home, ".specforge", "logs")
		_ = os.MkdirAll(logDir, 0755)
		globalLogger = &FileLogger{logDir: logDir}
	}
	return globalLogger
}

func (l *FileLogger) ensureFile() error {
	today := time.Now().Format("20060102")
	if l.file != nil && l.currentDay == today {
		return nil
	}

	if l.file != nil {
		_ = l.file.Close()
	}

	fileName := fmt.Sprintf("sdd-%s.log", today)
	filePath := filepath.Join(l.logDir, fileName)

	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	l.file = f
	l.currentDay = today
	return nil
}

// Log registra un mensaje con nivel y timestamp
func (l *FileLogger) Log(level LogLevel, format string, args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()

	msg := fmt.Sprintf(format, args...)
	timestamp := time.Now().Format("2006-01-02 15:04:05.000")
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, level, msg)

	if err := l.ensureFile(); err == nil && l.file != nil {
		_, _ = io.WriteString(l.file, line)
	}

	if (level == LevelDebug && l.debugMode) || (level == LevelInfo && l.verbose) || level == LevelError {
		_, _ = fmt.Fprint(os.Stderr, line)
	}
}

func (l *FileLogger) Debug(format string, args ...interface{}) {
	l.Log(LevelDebug, format, args...)
}

func (l *FileLogger) Info(format string, args ...interface{}) {
	l.Log(LevelInfo, format, args...)
}

func (l *FileLogger) Warn(format string, args ...interface{}) {
	l.Log(LevelWarn, format, args...)
}

func (l *FileLogger) Error(format string, args ...interface{}) {
	l.Log(LevelError, format, args...)
}

// LogIO registra silenciosamente prompts y salidas sin ensuciar la consola
func (l *FileLogger) LogIO(direction, stream, content string) {
	l.Log(LevelDebug, "[%s:%s]\n%s\n---", direction, stream, content)
}
