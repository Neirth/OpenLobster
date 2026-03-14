package logging

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Level int

const (
	DEBUG Level = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	level       Level
	logPath     string
	file        *os.File
	mu          sync.Mutex
	stdout      *os.File
	logBuffer   []string
	bufferSize  int
	bufferIndex int
}

var defaultLogger *Logger

func Init(path string, levelStr string) error {
	var level Level
	switch strings.ToLower(levelStr) {
	case "debug":
		level = DEBUG
	case "warn", "warning":
		level = WARN
	case "error":
		level = ERROR
	default:
		level = INFO
	}

	l := &Logger{
		level:       level,
		logPath:     path,
		bufferSize:  500,
		logBuffer:   make([]string, 500),
		bufferIndex: 0,
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}

	l.file = f
	l.stdout = os.Stdout

	mw := &multiWriter{file: f, stdout: os.Stdout, logger: l}
	log.SetOutput(mw)
	log.SetFlags(log.LstdFlags)

	defaultLogger = l

	return nil
}

type multiWriter struct {
	file   *os.File
	stdout *os.File
	logger *Logger
	mu     sync.Mutex
}

func (m *multiWriter) Write(p []byte) (n int, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Write to file and stdout.
	if m.file != nil {
		m.file.Write(p)
	}
	if m.stdout != nil {
		m.stdout.Write(p)
	}

	// Append to the in-memory circular buffer.
	if m.logger != nil {
		line := strings.TrimRight(string(p), "\n")
		if line != "" {
			m.logger.mu.Lock()
			m.logger.logBuffer[m.logger.bufferIndex] = line
			m.logger.bufferIndex = (m.logger.bufferIndex + 1) % m.logger.bufferSize
			m.logger.mu.Unlock()
		}
	}

	return len(p), nil
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

func (l *Logger) GetTailLines(n int) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if n <= 0 {
		return "", nil
	}

	// Clamp n to the buffer size.
	if n > l.bufferSize {
		n = l.bufferSize
	}

	// Collect the last n lines from the ring buffer.
	var lines []string
	for i := 0; i < n && i < l.bufferSize; i++ {
		// Compute index: walk backwards from bufferIndex.
		idx := (l.bufferIndex - 1 - i + l.bufferSize) % l.bufferSize
		line := l.logBuffer[idx]
		if line != "" {
			lines = append([]string{line}, lines...) // Insert at beginning
		}
	}

	return strings.Join(lines, "\n"), nil
}

func GetDefaultLogger() *Logger {
	return defaultLogger
}

func Close() error {
	if defaultLogger != nil {
		return defaultLogger.Close()
	}
	return nil
}
