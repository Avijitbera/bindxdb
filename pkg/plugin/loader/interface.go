package loader

import "bindxdb/pkg/plugin/types"

type PluginLoader interface {
	Load() (interface{}, error)
	Unload() error
	Metadata() (*types.PluginMetadata, error)
	Source() string
	Checksum() string
}
