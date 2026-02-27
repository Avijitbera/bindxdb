package main

import (
	"bindxdb/pkg/config"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	var (
		configFile = flag.String("config", "config.yaml", "Configuration file")
		command    = flag.String("cmd", "get", "Command: get, set, delete, list, watch, validate, reload")
		key        = flag.String("key", "", "Configuration key")
		value      = flag.String("value", "", "Configuration value")
		format     = flag.String("format", "yaml", "Output format (json, yaml)")
	)
	flag.Parse()

	if err := config.InitConfig([]string{*configFile}); err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize config: %v\n", err)
	}

	cfg := config.GetConfig()
	ctx := context.Background()

	switch *command {
	case "get":
		cmdGet(cfg, *key, *format)
	case "set":
		cmdSet(cfg, ctx, *key, *value, *format)
	case "delete":
		cmdDelete(cfg, ctx, *key)
	case "lsit":
		cmdList(cfg, *format)
	case "watch":
		cmdWatch(cfg, *key)
	case "validate":
		cmdValidate(cfg)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		os.Exit(1)
	}
}

func cmdSet(cfg *config.ConfigManager, ctx context.Context, key, value, format string) {
	var parsedValue interface{}

	if err := json.Unmarshal([]byte(value), &parsedValue); err != nil {
		parsedValue = value
	}

	if err := cfg.Set(key, parsedValue, config.SourceFlag, true); err != nil {
		fmt.Fprintf(os.Stderr, "failed to set config: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Config %s set successfully\n", key)
}

func cmdGet(cfg *config.ConfigManager, key, format string) {
	value, err := cfg.Get(key)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get config: %v\n", err)
		os.Exit(1)
	}
	output := map[string]interface{}{
		key: value,
	}
	printOutput(output, format)
}

func cmdDelete(cfg *config.ConfigManager, ctx context.Context, key string) {
	fmt.Println("Delete not implemented yet")
}
func cmdList(cfg *config.ConfigManager, format string) {
	fmt.Println("List not implemented yet")
}

func cmdWatch(cfg *config.ConfigManager, key string) {
	fmt.Printf("Watching config changes for %s...\n", key)
	fmt.Println("Press Ctrl+C to stop")

	ch := cfg.Watch()

	for change := range ch {
		if key == "" || change.Key == key || strings.HasPrefix(change.Key, key+".") {
			fmt.Printf("Config changed: %s = %v (from %s)\n",
				change.Key, change.NewValue, change.Source)
		}
	}

}

func cmdValidate(cfg *config.ConfigManager) {
	if err := cfg.ValidateAll(); err != nil {
		fmt.Fprintf(os.Stderr, "Validation failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration is valid")
}

func cmdReload(cfg *config.ConfigManager, ctx context.Context) {
	if err := cfg.Load(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to reload config: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Configuration reloaded")
}

func printOutput(data interface{}, format string) {
	switch format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", " ")
		enc.Encode(data)
	case "yaml":
		fallthrough
	default:
		fmt.Printf("%+v\n", data)
	}
}
