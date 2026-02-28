package profiling

import "time"

type Profiler interface {
	StartProfile(name string, tags map[string]string) *Profiler
}

type Profile struct {
	ID        string
	Name      string
	StartTime time.Time
	EndTime   time.Time
	Duration  time.Duration
	Tags      map[string]string
	Data      map[string]interface{}
	Children  []*Profile
	Parent    *Profile
}
