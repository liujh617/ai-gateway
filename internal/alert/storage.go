package alert

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// alertStorage handles persistent storage of alerts
type alertStorage struct {
	path         string
	maxFileBytes int64
	file         *os.File
	encoder      *json.Encoder
	mu           sync.Mutex
}

// newAlertStorage creates a new alert storage
func newAlertStorage(path string, maxFileBytes int64) (*alertStorage, error) {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create directory: %w", err)
	}

	// Open file for appending
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}

	// Check file size
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Rotate if needed
	if info.Size() >= maxFileBytes {
		file.Close()
		if err := rotateFile(path); err != nil {
			return nil, fmt.Errorf("rotate file: %w", err)
		}
		file, err = os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return nil, fmt.Errorf("open file after rotation: %w", err)
		}
	}

	return &alertStorage{
		path:         path,
		maxFileBytes: maxFileBytes,
		file:         file,
		encoder:      json.NewEncoder(file),
	}, nil
}

// Store stores an alert
func (s *alertStorage) Store(alert *Alert) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check file size
	info, err := s.file.Stat()
	if err != nil {
		return fmt.Errorf("stat file: %w", err)
	}

	// Rotate if needed
	if info.Size() >= s.maxFileBytes {
		if err := s.rotate(); err != nil {
			return fmt.Errorf("rotate: %w", err)
		}
	}

	// Write alert as JSON line
	if err := s.encoder.Encode(alert); err != nil {
		return fmt.Errorf("encode alert: %w", err)
	}

	return nil
}

// rotate rotates the log file
func (s *alertStorage) rotate() error {
	// Close current file
	if err := s.file.Close(); err != nil {
		return err
	}

	// Rename current file
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", s.path, timestamp)
	if err := os.Rename(s.path, rotatedPath); err != nil {
		return err
	}

	// Open new file
	file, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	s.file = file
	s.encoder = json.NewEncoder(file)
	return nil
}

// Close closes the storage
func (s *alertStorage) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.file != nil {
		return s.file.Close()
	}
	return nil
}

// rotateFile rotates a file externally
func rotateFile(path string) error {
	timestamp := time.Now().Format("20060102-150405")
	rotatedPath := fmt.Sprintf("%s.%s", path, timestamp)
	return os.Rename(path, rotatedPath)
}

// ReadAlerts reads alerts from a file
func ReadAlerts(path string) ([]*Alert, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var alerts []*Alert
	decoder := json.NewDecoder(file)

	for {
		var alert Alert
		if err := decoder.Decode(&alert); err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		alerts = append(alerts, &alert)
	}

	return alerts, nil
}