package config

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"
)

type DatabaseConfig struct {
	Host           string        `json:"host"`
	Port           int           `json:"port"`
	Name           string        `json:"name"`
	User           string        `json:"user"`
	Password       string        `json:"-"`
	MaxConnections int           `json:"max_connections"`
	IdleTimeout    time.Duration `json:"idle_timeout"`
	SSLMode        string        `json:"ssl_mode"`
}

type StorageConfig struct {
	Engine      string `json:"engine"`
	DataDir     string `json:"data_dir"`
	WALDir      string `json:"wal_dir"`
	PageSize    int    `json:"page_size"`
	CacheSize   int    `json:"cache_size"`
	SyncWrites  bool   `json:"sync_writes"`
	Compression string `json:"compression"`
}

type ServerConfig struct {
	HTTP      HTTPServerConfig      `json:"http"`
	GRPC      GRPCServerConfig      `json:"grpc"`
	Websocket WebsocketServerConfig `json:"websocket"`
}

type HTTPServerConfig struct {
	Enabled bool      `json:"enabled"`
	Port    int       `json:"port"`
	TLS     TLSConfig `json:"tls"`
}

type TLSConfig struct {
	Enabled  bool   `json:"enabled"`
	CertFile string `json:"cert_file"`
	KeyFile  string `json:"key_file"`
	CAFile   string `json:"ca_file"`
}

type GRPCServerConfig struct {
	Enabled bool      `json:"enabled"`
	Port    int       `json:"port"`
	TLS     TLSConfig `json:"tls"`
}

type WebsocketServerConfig struct {
	Enabled bool `json:"enabled"`
	Port    int  `json:"port"`
}

type AuthProviderConfig struct {
	Type   string                 `json:"type"`
	Config map[string]interface{} `json:"-"`
}

type LoggingConfig struct {
	Level      string        `json:"level"`
	Format     string        `json:"format"`
	Output     string        `json:"output"`
	File       string        `json:"file"`
	MaxSize    string        `json:"max_size"`
	MaxBackups int           `json:"max_backups"`
	MaxAge     time.Duration `json:"max_age"`
}

type MetricsConfig struct {
	Enabled    bool             `json:"enabled"`
	Prometheus PrometheusConfig `json:"prometheus"`
	Graphite   GraphiteConfig   `json:"graphite"`
}

type PrometheusConfig struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
}

type GraphiteConfig struct {
	Enabled bool   `json:"enabled"`
	Host    string `json:"host"`
	Port    int    `json:"port"`
	Prefix  string `json:"prefix"`
}

type PluginConfig struct {
	Directory string                 `json:"directory"`
	AutoLoad  bool                   `json:"auto_load"`
	Enabled   []string               `json:"enabled"`
	Configs   map[string]interface{} `json:"configs"`
}

type AppConfig struct {
	Database DatabaseConfig `json:"database"`
	Storage  StorageConfig  `json:"storage"`
	Server   ServerConfig   `json:"server"`
	Auth     struct {
		Providers []AuthProviderConfig `json:"providers"`
		RBAC      struct {
			Enabled   bool   `json:"enabled"`
			RolesPath string `json:"roles_path"`
		} `json:"rbac"`
	} `json:"auth"`
	Logging LoggingConfig `json:"logging"`
	Metrics MetricsConfig `json:"metrics"`
	Plugins PluginConfig  `json:"plugins"`
}

var (
	globalManager *ConfigManager
	globalOnce    sync.Once
)

func InitConfig(configPaths []string) error {
	var err error
	globalOnce.Do(func() {
		logger := &DefaultLogger{}

		secretStore, err := createSecretStore()

		if err != nil {
			return
		}

		manager := NewConfigManager(logger, secretStore)

		fileSource := NewFileSource(configPaths, 50)
		if err := manager.AddSource(fileSource); err != nil {
			return
		}

		envSource := NewEnvironmentSource("BINDXDB_", 75)
		if err := manager.AddSource(envSource); err != nil {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		setDefault(manager)
		addValidators(manager)

		if err := manager.Load(ctx); err != nil {
			return
		}

		globalManager = manager

	})
	return err
}

func GetConfig() *ConfigManager {
	return globalManager
}

func GetAppConfig() (*AppConfig, error) {
	if globalManager == nil {
		return nil, fmt.Errorf("configuration not initialized")
	}
	var appConfig AppConfig
	appConfig.Database.Host, _ = globalManager.GetString("database.host")
	appConfig.Database.Port, _ = globalManager.GetInt("database.port")
	appConfig.Database.Name, _ = globalManager.GetString("database.name")
	appConfig.Database.User, _ = globalManager.GetString("database.user")
	appConfig.Database.Password, _ = globalManager.GetString("database.password")
	appConfig.Database.MaxConnections, _ = globalManager.GetInt("database.max_connections")
	appConfig.Database.IdleTimeout, _ = globalManager.GetDuration("database.idle_timeout")
	appConfig.Database.SSLMode, _ = globalManager.GetString("database.ssl_mode")

	appConfig.Storage.Engine, _ = globalManager.GetString("storage.engine")
	appConfig.Storage.DataDir, _ = globalManager.GetString("storage.engine")
	appConfig.Storage.WALDir, _ = globalManager.GetString("storage.wal_dir")
	appConfig.Storage.PageSize, _ = globalManager.GetInt("storage.page_size")
	appConfig.Storage.SyncWrites, _ = globalManager.GetBool("storage.sync_writes")
	appConfig.Storage.Compression, _ = globalManager.GetString("storage.compression")

	appConfig.Server.HTTP.Enabled, _ = globalManager.GetBool("server.http.enabled")
	appConfig.Server.HTTP.Port, _ = globalManager.GetInt("server.http.port")
	appConfig.Server.HTTP.TLS.Enabled, _ = globalManager.GetBool("server.http.tls.enabled")
	appConfig.Server.HTTP.TLS.CertFile, _ = globalManager.GetString("server.http.tls.cert_file")
	appConfig.Server.HTTP.TLS.KeyFile, _ = globalManager.GetString("server.http.tls.key_file")

	appConfig.Logging.Level, _ = globalManager.GetString("logging.level")
	appConfig.Logging.Format, _ = globalManager.GetString("logging.format")
	appConfig.Logging.Output, _ = globalManager.GetString("logging.output")
	appConfig.Logging.File, _ = globalManager.GetString("logging.file")
	appConfig.Logging.MaxBackups, _ = globalManager.GetInt("logging.max_backups")
	appConfig.Logging.MaxAge, _ = globalManager.GetDuration("logging.max_age")

	appConfig.Plugins.Directory, _ = globalManager.GetString("plugins.directory")
	appConfig.Plugins.AutoLoad, _ = globalManager.GetBool("plugins.auto_load")
	appConfig.Plugins.Enabled, _ = globalManager.GetStringSlice("plugins.enabled")

	return &appConfig, nil
}

func setDefault(manager *ConfigManager) {
	manager.SetDefault("database.host", "localhost")
	manager.SetDefault("database.port", 5432)

	manager.SetDefault("database.max_connections", 10)
	manager.SetDefault("database.idle_timeout", 5*time.Minute)
	manager.SetDefault("database.ssl_mode", "prefer")

	manager.SetDefault("storage.engine", "btree")
	manager.SetDefault("storage.page_size", 8192)
	manager.SetDefault("storage.cache_size", 1024)
	manager.SetDefault("storage.sync_writes", true)

	manager.SetDefault("logging.level", "info")
	manager.SetDefault("logging.format", "json")
	manager.SetDefault("logging.output", "stdout")
	manager.SetDefault("logging.max_backups", 10)
	manager.SetDefault("logging.max_age", "30d")

	manager.SetDefault("metrics.enabled", true)
	manager.SetDefault("metrics.prometheus.enabled", true)
	manager.SetDefault("metrics.prometheus.path", "/metrics")

	manager.SetDefault("plugins.directory", "/usr/lib/bindxdb/plugins")
	manager.SetDefault("plugins.auto_load", true)

}

func addValidators(manager *ConfigManager) {
	portValidator := &PortValidator{Min: 1, Max: 65535}
	manager.AddValidator("database.port", portValidator)
	manager.AddValidator("server.http.port", portValidator)
	manager.AddValidator("server.grpc.port", portValidator)

	requiredValidator := &RequiredValidator{}
	manager.AddValidator("database.name", requiredValidator)
	manager.AddValidator("storage.data_dir", requiredValidator)

	fileValidator := &FileValidator{MustExist: true, MustBeDir: true}
	manager.AddValidator("storage.data_dir", fileValidator)

	durationValidator := &DurationValidator{Min: 1 * time.Second}
	manager.AddValidator("database.idle_timeout", durationValidator)

	if hostnameValidator, err := NewPatternValidator(`^[a-zA-Z0-9\.\-]+$`); err != nil {
		manager.AddValidator("database.host", hostnameValidator)
	}

}

func createSecretStore() (SecretStore, error) {
	encKey := os.Getenv("BINDXDB_ENCRYPTION_KEY")
	if encKey == "" {
		encKey = ""
	}
	encryption, err := NewAESEncryption([]byte(encKey))
	if err != nil {
		return nil, err
	}

	secretDir := os.Getenv("BINDXDB_SECRET_DIR")
	if secretDir == "" {
		secretDir = "/etc/bindxdb/secrets"
	}

	return NewFileSecretStore(secretDir, encryption, &DefaultLogger{})
}

type DefaultLogger struct{}

func (l *DefaultLogger) Debug(msg string, args ...interface{}) {
	fmt.Printf("[DEBUG] "+msg+"\n", args...)
}

func (l *DefaultLogger) Info(msg string, args ...interface{}) {
	fmt.Printf("[INFO] "+msg+"\n", args...)
}

func (l *DefaultLogger) Warn(msg string, args ...interface{}) {
	fmt.Printf("[WARN] "+msg+"\n", args...)
}

func (l *DefaultLogger) Error(msg string, args ...interface{}) {
	fmt.Printf("[ERROR] "+msg+"\n", args...)
}
