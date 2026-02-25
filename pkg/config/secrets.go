package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type SecretStores interface {
	GetSecret(key string) (string, error)
	SetSecret(key string, value string) error
	DeleteSecret(key string) error
	ListSecrets() ([]string, error)
}

type FileSecretStore struct {
	basePath   string
	encryption Encryption
	cache      map[string]cachedSecret
	mu         sync.RWMutex
	logger     Logger
}

type cachedSecret struct {
	value     string
	expiresAt time.Time
}

type Encryption interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

type AESEncryption struct {
	key []byte
}

func NewAESEncryption(key []byte) (*AESEncryption, error) {
	if len(key) != 32 {
		hash := sha256.Sum256(key)
		key = hash[:]
	}
	return &AESEncryption{
		key: key,
	}, nil
}

func (e *AESEncryption) Encrypt(plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

func (e *AESEncryption) Decrypt(ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(e.key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}
	return plaintext, nil

}

func NewFileSecretStore(basePath string, encryption Encryption, logger Logger) (*FileSecretStore, error) {
	if err := os.MkdirAll(basePath, 0700); err != nil {
		return nil, fmt.Errorf("failed to create secret store directory: %w", err)
	}

	return &FileSecretStore{
		basePath:   basePath,
		encryption: encryption,
		cache:      make(map[string]cachedSecret),
		logger:     logger,
	}, nil
}

func (s *FileSecretStore) GetSecret(key string) (string, error) {
	s.mu.RLock()
	cached, exists := s.cache[key]
	s.mu.RUnlock()

	if exists && cached.expiresAt.After(time.Now()) {
		return cached.value, nil
	}

	filePath := filepath.Join(s.basePath, sanitizeKey(key)+".enc")
	data, err := ioutil.ReadFile(filePath)

	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("secret %s not found", key)
		}
		return "", fmt.Errorf("failed to read secret file: %w", err)
	}

	ciphertext := make([]byte, base64.StdEncoding.DecodedLen(len(data)))
	n, err := base64.StdEncoding.Decode(ciphertext, data)
	if err != nil {
		return "", fmt.Errorf("failed to decode based64: %w", err)
	}
	ciphertext = ciphertext[:n]

	plaintext, err := s.encryption.Decrypt(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt secret: %w", err)
	}
	value := string(plaintext)

	s.mu.Lock()
	s.cache[key] = cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()

	return value, nil
}

func (s *FileSecretStore) SetSecret(key string, value string) error {
	ciphertext, err := s.encryption.Encrypt([]byte(value))
	if err != nil {
		return fmt.Errorf("failed to encrypt secret: %w", err)
	}

	encoded := make([]byte, base64.StdEncoding.EncodedLen(len(ciphertext)))
	base64.StdEncoding.Encode(encoded, ciphertext)

	filePath := filepath.Join(s.basePath, sanitizeKey(key)+".enc")
	if err := ioutil.WriteFile(filePath, encoded, 0600); err != nil {
		return fmt.Errorf("failed to write secret file: %w", err)
	}

	s.mu.Lock()
	s.cache[key] = cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()

	return nil
}

func (s *FileSecretStore) DeleteSecret(key string) error {
	filePath := filepath.Join(s.basePath, sanitizeKey(key)+".enc")

	if err := os.Remove(filePath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("secret %s not found", key)
		}
		return fmt.Errorf("failed to delete secret file: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()

	return nil
}

func sanitizeKey(key string) string {
	replacer := strings.NewReplacer(
		"/", "_",
		"\\", "_",
		":", "_",
		"*", "_",
		"?", "_",
		"\"", "_",
		"<", "_",
		">", "_",
		"|", "_",
	)
	return replacer.Replace(key)
}
