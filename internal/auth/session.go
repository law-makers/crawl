// internal/auth/session.go
package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
)

const (
	// KeyringService is the service name for keyring storage
	KeyringService = "crawl-cli"
	// FallbackDir is the directory for file-based session storage (when keyring fails)
	FallbackDir = ".crawl/sessions"
)

// useFileBasedStorage checks if we should use file-based storage
// This is a fallback for environments where keyring isn't available (Codespaces, CI)
var fileBasedStorageCache *bool

func useFileBasedStorage() bool {
	// Cache the result to avoid repeated tests
	if fileBasedStorageCache != nil {
		return *fileBasedStorageCache
	}

	// Check environment hints
	if os.Getenv("CODESPACES") != "" || os.Getenv("CI") != "" {
		result := true
		fileBasedStorageCache = &result
		return true
	}

	// Try to use keyring, but if it fails, use file-based storage
	testKey := "_test_keyring_access_"
	err := keyring.Set(KeyringService, testKey, "test")
	result := (err != nil)
	fileBasedStorageCache = &result

	if !result {
		keyring.Delete(KeyringService, testKey)
	}

	return result
}

// getSessionDir returns the session storage directory
func getSessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, FallbackDir)
	return dir, os.MkdirAll(dir, 0700)
}

// getSessionPath returns the file path for a session
func getSessionPath(name string) (string, error) {
	dir, err := getSessionDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name+".json"), nil
}

// SessionData represents stored authentication session
type SessionData struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Cookies   []Cookie          `json:"cookies"`
	Headers   map[string]string `json:"headers,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	ExpiresAt time.Time         `json:"expires_at,omitempty"`
}

// Cookie represents a browser cookie
type Cookie struct {
	Name     string  `json:"name"`
	Value    string  `json:"value"`
	Domain   string  `json:"domain"`
	Path     string  `json:"path"`
	Expires  float64 `json:"expires"`
	HTTPOnly bool    `json:"httpOnly"`
	Secure   bool    `json:"secure"`
	SameSite string  `json:"sameSite,omitempty"`
}

// SaveSession saves a session securely to the OS keyring or file
func SaveSession(session *SessionData) error {
	if session.Name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Serialize session data
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to serialize session: %w", err)
	}

	// Try keyring first, fallback to file
	if useFileBasedStorage() {
		path, err := getSessionPath(session.Name)
		if err != nil {
			return fmt.Errorf("failed to get session path: %w", err)
		}
		err = os.WriteFile(path, data, 0600)
		if err != nil {
			return fmt.Errorf("failed to save session file: %w", err)
		}
		return nil
	}

	// Store in keyring (encrypted by OS)
	err = keyring.Set(KeyringService, session.Name, string(data))
	if err != nil {
		return fmt.Errorf("failed to save to keyring: %w", err)
	}

	return nil
}

// LoadSession loads a session from the OS keyring or file
func LoadSession(name string) (*SessionData, error) {
	if name == "" {
		return nil, fmt.Errorf("session name cannot be empty")
	}

	var data string
	var err error

	// Try keyring first, fallback to file
	if useFileBasedStorage() {
		path, err := getSessionPath(name)
		if err != nil {
			return nil, fmt.Errorf("failed to get session path: %w", err)
		}
		fileData, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to load session file: %w", err)
		}
		data = string(fileData)
	} else {
		// Retrieve from keyring
		data, err = keyring.Get(KeyringService, name)
		if err != nil {
			return nil, fmt.Errorf("failed to load from keyring: %w", err)
		}
	}

	// Deserialize
	var session SessionData
	if err := json.Unmarshal([]byte(data), &session); err != nil {
		return nil, fmt.Errorf("failed to deserialize session: %w", err)
	}

	// Check if expired
	if !session.ExpiresAt.IsZero() && time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired")
	}

	return &session, nil
}

// DeleteSession removes a session from the OS keyring or file
func DeleteSession(name string) error {
	if name == "" {
		return fmt.Errorf("session name cannot be empty")
	}

	// Try keyring first, fallback to file
	if useFileBasedStorage() {
		path, err := getSessionPath(name)
		if err != nil {
			return fmt.Errorf("failed to get session path: %w", err)
		}
		err = os.Remove(path)
		if err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to delete session file: %w", err)
		}
		return nil
	}

	err := keyring.Delete(KeyringService, name)
	if err != nil {
		return fmt.Errorf("failed to delete from keyring: %w", err)
	}

	return nil
}

// ListSessions returns a list of all stored session names
func ListSessions() ([]string, error) {
	// Try keyring first, fallback to file
	if useFileBasedStorage() {
		dir, err := getSessionDir()
		if err != nil {
			return nil, err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			if os.IsNotExist(err) {
				return []string{}, nil
			}
			return nil, err
		}

		var sessions []string
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				name := strings.TrimSuffix(entry.Name(), ".json")
				sessions = append(sessions, name)
			}
		}
		return sessions, nil
	}

	// Try to load the manifest from keyring
	manifestData, err := keyring.Get(KeyringService, "_manifest")
	if err != nil {
		// No manifest exists yet
		return []string{}, nil
	}

	var sessions []string
	if err := json.Unmarshal([]byte(manifestData), &sessions); err != nil {
		return nil, fmt.Errorf("failed to deserialize manifest: %w", err)
	}

	return sessions, nil
}

// updateManifest adds or removes a session from the manifest
func updateManifest(sessionName string, add bool) error {
	sessions, _ := ListSessions()

	if add {
		// Add to manifest if not already present
		found := false
		for _, s := range sessions {
			if s == sessionName {
				found = true
				break
			}
		}
		if !found {
			sessions = append(sessions, sessionName)
		}
	} else {
		// Remove from manifest
		newSessions := []string{}
		for _, s := range sessions {
			if s != sessionName {
				newSessions = append(newSessions, s)
			}
		}
		sessions = newSessions
	}

	// Save manifest
	data, err := json.Marshal(sessions)
	if err != nil {
		return err
	}

	return keyring.Set(KeyringService, "_manifest", string(data))
}

// SaveSessionWithManifest saves a session and updates the manifest
func SaveSessionWithManifest(session *SessionData) error {
	if err := SaveSession(session); err != nil {
		return err
	}

	// Skip manifest for file-based storage (uses directory listing instead)
	if useFileBasedStorage() {
		return nil
	}

	return updateManifest(session.Name, true)
}

// DeleteSessionWithManifest deletes a session and updates the manifest
func DeleteSessionWithManifest(name string) error {
	if err := DeleteSession(name); err != nil {
		return err
	}

	// Skip manifest for file-based storage (uses directory listing instead)
	if useFileBasedStorage() {
		return nil
	}

	return updateManifest(name, false)
}
