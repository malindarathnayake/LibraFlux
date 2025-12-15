package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	"gopkg.in/yaml.v3"
)

// EnvVarRegex matches ${VAR_NAME}
var EnvVarRegex = regexp.MustCompile(`\$\{([A-Z0-9_]+)\}`)

// LoadConfig loads the configuration from the specified path
func LoadConfig(path string) (*Config, error) {
	// 1. Read main config
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// 2. Resolve environment variables
	resolvedData, err := ResolveEnvVars(data)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve env vars: %w", err)
	}

	// 3. Enforce that the main config contains globals only (no services).
	var mainTop map[string]interface{}
	if err := yaml.Unmarshal(resolvedData, &mainTop); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}
	if _, ok := mainTop["services"]; ok {
		return nil, fmt.Errorf("main config must not define services; define services in config.d files")
	}

	// 4. Unmarshal main config
	var cfg Config
	if err := yaml.Unmarshal(resolvedData, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// 5. Handle includes
	if cfg.Include != "" {
		// Resolve include path relative to config file if not absolute
		includePattern := cfg.Include
		if !filepath.IsAbs(includePattern) {
			includePattern = filepath.Join(filepath.Dir(path), includePattern)
		}

		matches, err := filepath.Glob(includePattern)
		if err != nil {
			return nil, fmt.Errorf("failed to glob include pattern: %w", err)
		}

		sort.Strings(matches) // Alphabetical order

		for _, match := range matches {
			if err := loadServiceConfig(match, &cfg); err != nil {
				return nil, fmt.Errorf("failed to load service config %s: %w", match, err)
			}
		}
	}

	return &cfg, nil
}

// loadServiceConfig loads a service configuration file and appends to the main config
func loadServiceConfig(path string, cfg *Config) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	resolvedData, err := ResolveEnvVars(data)
	if err != nil {
		return err
	}

	// Enforce that config.d files contain only `services` at the top level (per spec).
	var top map[string]interface{}
	if err := yaml.Unmarshal(resolvedData, &top); err != nil {
		return err
	}
	if len(top) == 0 {
		return nil
	}
	if len(top) != 1 {
		return fmt.Errorf("service config file must contain only 'services'")
	}
	if _, ok := top["services"]; !ok {
		return fmt.Errorf("service config file must contain only 'services'")
	}

	var serviceCfg ServiceConfig
	if err := yaml.Unmarshal(resolvedData, &serviceCfg); err != nil {
		return err
	}

	// Merge services
	if len(serviceCfg.Services) > 0 {
		cfg.Services = append(cfg.Services, serviceCfg.Services...)
	}

	return nil
}

// ResolveEnvVars replaces ${VAR} with environment variable values
func ResolveEnvVars(data []byte) ([]byte, error) {
	content := string(data)
	var missingVars []string

	// First pass: check for missing variables
	matches := EnvVarRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		varName := match[1]
		if _, ok := os.LookupEnv(varName); !ok {
			// Check if already added to missingVars to avoid duplicates
			found := false
			for _, v := range missingVars {
				if v == varName {
					found = true
					break
				}
			}
			if !found {
				missingVars = append(missingVars, varName)
			}
		}
	}

	if len(missingVars) > 0 {
		return nil, fmt.Errorf("missing environment variables: %v", missingVars)
	}

	// Second pass: replace
	resolved := EnvVarRegex.ReplaceAllStringFunc(content, func(match string) string {
		varName := match[2 : len(match)-1] // Remove ${ and }
		val, _ := os.LookupEnv(varName)
		return val
	})

	return []byte(resolved), nil
}
