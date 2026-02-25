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

	vault "github.com/hashicorp/vault/api"
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

func (s *FileSecretStore) ListSecrets() ([]string, error) {
	files, err := ioutil.ReadDir(s.basePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read secret store directory: %w", err)
	}

	var secrets []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".enc" {
			key := file.Name()[:len(file.Name())-4]
			secrets = append(secrets, key)
		}
	}

	return secrets, nil
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

type VaultSecretStore struct {
	client    *vault.Client
	mountPath string
	cache     map[string]cachedSecret
	mu        sync.RWMutex
	logger    Logger
}

func NewVaultSecretStore(address, token, mountPath string, logger Logger) (*VaultSecretStore, error) {
	config := vault.DefaultConfig()
	config.Address = address
	client, err := vault.NewClient(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Vault client: %w", err)
	}
	client.SetToken(token)
	return &VaultSecretStore{
		client:    client,
		mountPath: mountPath,
		cache:     make(map[string]cachedSecret),
		logger:    logger,
	}, nil
}

func (s *VaultSecretStore) GetSecret(key string) (string, error) {
	s.mu.RLock()
	cached, exists := s.cache[key]
	s.mu.RUnlock()
	if exists && cached.expiresAt.After(time.Now()) {
		return cached.value, nil
	}

	secret, err := s.client.Logical().Read(fmt.Sprintf("%s/data/%s", s.mountPath, key))
	if err != nil {
		return "", fmt.Errorf("failed to read from Vault: %w", err)
	}
	if secret == nil || secret.Data == nil {
		return "", fmt.Errorf("secret %s not found", key)
	}
	data, ok := secret.Data["data"].(map[string]interface{})
	if !ok {
		return "", fmt.Errorf("unexpected Vault response format")
	}
	value, ok := data["value"].(string)
	if !ok {
		return "", fmt.Errorf("secret value not found or not a string")
	}
	s.mu.Lock()
	s.cache[key] = cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()
	return value, nil
}

func (s *VaultSecretStore) SetSecret(key string, value string) error {
	data := map[string]interface{}{
		"data": map[string]interface{}{
			"value": value,
		},
	}

	_, err := s.client.Logical().Write(fmt.Sprintf("%s/data/%s", s.mountPath, key), data)
	if err != nil {
		return fmt.Errorf("failed to write to Vault: %w", err)
	}

	s.mu.Lock()
	s.cache[key] = cachedSecret{
		value:     value,
		expiresAt: time.Now().Add(5 * time.Minute),
	}
	s.mu.Unlock()
	return nil
}

func (s *VaultSecretStore) DeleteSecret(key string) error {
	_, err := s.client.Logical().Delete(fmt.Sprintf("%s/data/%s", s.mountPath, key))
	if err != nil {
		return fmt.Errorf("failed to delete from Vault: %w", err)
	}
	s.mu.Lock()
	delete(s.cache, key)
	s.mu.Unlock()
	return nil
}

func (s *VaultSecretStore) ListSecrets() ([]string, error) {
	secret, err := s.client.Logical().List(fmt.Sprintf("%s/metadata", s.mountPath))
	if err != nil {
		return nil, fmt.Errorf("failed to list secrets from Vault: %w", err)
	}

	if secret == nil || secret.Data == nil {
		return nil, nil
	}

	keys, ok := secret.Data["keys"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected Vault response format")
	}

	result := make([]string, len(keys))
	for i, key := range keys {
		result[i] = key.(string)
	}
	return result, nil

}
