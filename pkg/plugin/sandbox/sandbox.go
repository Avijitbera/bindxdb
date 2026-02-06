package sandbox

import (
	"context"
	"fmt"
	"os"
	"runtime"
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

func (s *Sandbox) Execute(ctx context.Context, fn func() error) error {
	s.mu.Lock()
	s.stratedAt = time.Now()

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.mu.Unlock()

	defer cancel()

	monitorDone := make(chan struct{})

	go s.monitorResources(ctx, monitorDone)

	errCh := make(chan error, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("plugin panic: %v", r)
			}
		}()

		if err := s.applyLimits(); err != nil {
			errCh <- err
			return
		}
		errCh <- fn()
	}()

	var err error
	select {
	case err = <-errCh:
		// function completed
	case <-ctx.Done():
		err = fmt.Errorf("execution cancelled: %w", ctx.Err())
	case <-time.After(s.limits.MaxCPUTime):
		err = fmt.Errorf("CPU time limit exceeded: %v", s.limits.MaxCPUTime)
	}

	close(monitorDone)

	return err

}

func (s *Sandbox) applyLimits() error {
	if s.limits.MaxMemory > 0 {
		if err := setMemoryLimit(s.limits.MaxMemory); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
	}

	if s.limits.MaxOpenFiles > 0 {
		var rlimit syscall.Rlimit
		if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
			return fmt.Errorf("failed to get file descriptor limit: %w", err)
		}
		rlimit.Cur = uint64(s.limits.MaxOpenFiles)
		if rlimit.Cur > rlimit.Max {
			rlimit.Cur = rlimit.Max
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
			return fmt.Errorf("failed to set file limit: %w", err)
		}
	}
	return nil
}

func (s *Sandbox) monitorResources(ctx context.Context, done chan struct{}) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			s.updateResourceUsage()
			if s.exceedsLimits() {
				s.mu.RLock()
				cancel := s.cancel
				s.mu.RUnlock()
				if cancel != nil {
					cancel()
				}
				return
			}
		}
	}
}

func (s *Sandbox) updateResourceUsage() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		s.cpuUsage = time.Duration(usage.Utime.Nano() + usage.Stime.Nano())
	}

	var mstats runtime.MemStats
	runtime.ReadMemStats(&mstats)
	s.memoryUsage = int64(mstats.Alloc)
	s.openFiles = countOpenFiles()
}

func (s *Sandbox) exceedsLimits() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.cpuUsage > s.limits.MaxCPUTime {
		return true
	}

	if s.memoryUsage > s.limits.MaxMemory && s.limits.MaxMemory > 0 {
		return true
	}
	if s.openFiles > s.limits.MaxOpenFiles && s.limits.MaxOpenFiles > 0 {
		return true
	}
	return false
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
