package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// WriteServiceConfig writes a service configuration to a YAML file in the specified directory
func WriteServiceConfig(dir string, svc Service) error {
	// Validate service first
	if err := validateSingleService(svc); err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Create struct for marshalling (wrapped in services array as per spec)
	cfg := ServiceConfig{
		Services: []Service{svc},
	}

	data, err := yaml.Marshal(&cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal service config: %w", err)
	}

	filename := fmt.Sprintf("%s.yaml", svc.Name)
	path := filepath.Join(dir, filename)

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write service config file: %w", err)
	}

	return nil
}

// validateSingleService reuses validation logic for a single service
func validateSingleService(svc Service) error {
	// Wrap in Config for reused validation function
	dummyCfg := &Config{
		Services: []Service{svc},
	}
	return validateServices(dummyCfg)
}
