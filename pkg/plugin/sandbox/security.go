package sandbox

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type Permission string

const (
	PermissionFileRead  Permission = "file.read"
	PermissionFileWrite Permission = "file.write"
	PermissionNetwork   Permission = "network"
	PermissionSyscall   Permission = "syscall"
	PermissionExec      Permission = "exec"
	PermissionEnv       Permission = "env"
)

type SecurityPolicy struct {
	PluginID     string
	Permissions  []Permission
	AllowedPaths []string
	AllowedHosts []string
	AllowedPorts []string
	MaxMemory    int64
	MaxCPUTime   time.Duration
	AllowNetwork bool
	AllowExec    bool
	AllowEnv     bool
}

type SecurityManager struct {
	mu            sync.RWMutex
	policies      map[string]*SecurityPolicy
	defaultPolicy *SecurityPolicy
	signatures    map[string]string
}

func NewSecurityManager() *SecurityManager {
	defaultPolicy := &SecurityPolicy{
		Permissions: []Permission{PermissionFileRead},
		AllowedPaths: []string{
			".",
			"/tmp",
		},
		MaxMemory:    100 * 1024 * 1024,
		MaxCPUTime:   10 * time.Second,
		AllowNetwork: false,
		AllowExec:    false,
		AllowEnv:     false,
	}

	return &SecurityManager{
		policies:      make(map[string]*SecurityPolicy),
		defaultPolicy: defaultPolicy,
		signatures:    make(map[string]string),
	}
}

func (sm *SecurityManager) SetPolicy(policy *SecurityPolicy) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.validatePolicy(policy); err != nil {
		return err
	}
	sm.policies[policy.PluginID] = policy
	return nil

}

func (sm *SecurityManager) validatePolicy(policy *SecurityPolicy) error {
	if policy.PluginID == "" {
		return fmt.Errorf("plugin ID is required")

	}

	for _, path := range policy.AllowedPaths {
		if !filepath.IsAbs(path) && path != "." {
			return fmt.Errorf("path must be absolute: %s", path)
		}
	}

	if policy.MaxMemory <= 0 {
		return fmt.Errorf("max memory must be positive")
	}

	if policy.MaxCPUTime <= 0 {
		return fmt.Errorf("max CPU time must be positive")
	}
	return nil
}

func (sm *SecurityManager) GetPolicy(pluginID string) *SecurityPolicy {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	if policy, exists := sm.policies[pluginID]; exists {
		return policy
	}

	defaultCopy := *sm.defaultPolicy
	defaultCopy.PluginID = pluginID
	return &defaultCopy
}

func (sm *SecurityManager) CheckPermission(pluginID string, perm Permission) bool {
	policy := sm.GetPolicy(pluginID)

	for _, p := range policy.Permissions {
		if p == perm {
			return true
		}
	}
	return false
}

func (sm *SecurityManager) VerifyPlugin(pluginPath string, expectedChecksum string) (bool, error) {
	data, err := os.ReadFile(pluginPath)

	if err != nil {
		return false, err
	}

	hash := sha256.Sum256(data)

	actualChecksum := hex.EncodeToString(hash[:])

	if expectedChecksum != "" && actualChecksum != expectedChecksum {
		return false, fmt.Errorf("plugin checksum mismatch: expected %s, got %s", expectedChecksum, actualChecksum)
	}

	pluginID := filepath.Base(pluginPath)
	pluginID = strings.TrimSuffix(pluginID, filepath.Ext(pluginID))

	sm.mu.Lock()
	sm.signatures[pluginID] = actualChecksum
	sm.mu.Unlock()

	return true, nil
}

func (sm *SecurityManager) GetSignature(pluginID string) (string, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	signature, exists := sm.signatures[pluginID]
	if !exists {
		return "", fmt.Errorf("no signature found for plugin %s", pluginID)
	}
	return signature, nil
}

func (sm *SecurityManager) isPathAllowed(path string, allowedPaths []string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	for _, allowedPath := range allowedPaths {
		allowedAbs, err := filepath.Abs(allowedPath)
		if err != nil {
			continue
		}

		rel, err := filepath.Rel(allowedAbs, absPath)
		if err != nil {
			continue
		}
		if !strings.Contains(rel, "..") {
			return true
		}
	}
	return false
}
