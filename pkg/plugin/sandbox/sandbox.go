package sandbox

import (
	"context"
	"os"
	"sync"
	"syscall"
	"time"
)

type ResourceLimits struct {
	MaxCPUTime    time.Duration
	MaxMemory     int64
	MaxOpenFiles  int
	MaxThreads    int
	MaxChildProcs int
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxCPUTime:    10 * time.Second,
		MaxMemory:     100 * 1024 * 1024,
		MaxOpenFiles:  100,
		MaxThreads:    50,
		MaxChildProcs: 0,
	}
}

type Sandbox struct {
	pluginID  string
	limits    ResourceLimits
	stratedAt time.Time

	cpuUsage    time.Duration
	memoryUsage int64
	openFiles   int

	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.RWMutex
}

func NewSandbox(pluginID string, limits ResourceLimits) *Sandbox {
	return &Sandbox{
		pluginID: pluginID,
		limits:   limits,
		done:     make(chan struct{}),
	}
}

// Set the memory limit for the sandbox
func setMemoryLimit(limit int64) error {
	var rlimit syscall.Rlimit
	rlimit.Cur = uint64(limit)
	rlimit.Max = uint64(limit)

	return syscall.Setrlimit(syscall.RLIMIT_AS, &rlimit)
}

func countOpenFiles() int {
	dir, err := os.Open("/proc/self/fd")
	if err != nil {
		return 0
	}
	defer dir.Close()

	files, err := dir.Readdirnames(-1)
	if err != nil {
		return 0
	}

	return len(files) - 1
}
