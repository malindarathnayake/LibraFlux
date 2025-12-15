package shell

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/malindarathnayake/LibraFlux/internal/config"
)

type ConfigMode struct {
	configPath  string
	configDir   string
	idleTimeout time.Duration

	lock    *HeldLock
	base    *config.Config
	staged  map[string]config.Service
	deleted map[string]bool
}

func NewConfigMode(configPath, configDir string, idleTimeout time.Duration, lock *HeldLock) (*ConfigMode, error) {
	if lock == nil {
		return nil, errors.New("lock is required")
	}
	base, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, err
	}
	return &ConfigMode{
		configPath:  configPath,
		configDir:   configDir,
		idleTimeout: idleTimeout,
		lock:        lock,
		base:        base,
		staged:      make(map[string]config.Service),
		deleted:     make(map[string]bool),
	}, nil
}

func (m *ConfigMode) EnterService(name string) (*ServiceMode, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("service name required")
	}

	if m.deleted[name] {
		delete(m.deleted, name)
	}

	if svc, ok := m.staged[name]; ok {
		return NewServiceMode(svc)
	}
	for _, svc := range m.base.Services {
		if svc.Name == name {
			return NewServiceMode(svc)
		}
	}
	return NewServiceMode(config.Service{
		Name:      name,
		Protocol:  "tcp",
		Scheduler: "rr",
		Health: config.HealthCheck{
			Enabled: false,
			Type:    "tcp",
		},
	})
}

func (m *ConfigMode) StageService(svc config.Service) error {
	if strings.TrimSpace(svc.Name) == "" {
		return errors.New("service name required")
	}
	m.staged[svc.Name] = svc
	delete(m.deleted, svc.Name)
	return nil
}

func (m *ConfigMode) DeleteService(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("service name required")
	}
	delete(m.staged, name)
	m.deleted[name] = true
	return nil
}

func (m *ConfigMode) Abort(s *Shell) error {
	m.staged = make(map[string]config.Service)
	m.deleted = make(map[string]bool)
	fmt.Fprintln(s.out, "Aborted pending changes.")
	return nil
}

func (m *ConfigMode) ShowPending(s *Shell) error {
	added, updated := m.diff()
	if len(added) == 0 && len(updated) == 0 && len(m.deleted) == 0 {
		fmt.Fprintln(s.out, "No pending changes.")
		return nil
	}
	fmt.Fprintln(s.out, "Pending changes")
	for _, n := range added {
		fmt.Fprintf(s.out, "  + service %s (new)\n", n)
	}
	for _, n := range updated {
		fmt.Fprintf(s.out, "  ~ service %s (modified)\n", n)
	}
	var deleted []string
	for n := range m.deleted {
		deleted = append(deleted, n)
	}
	sort.Strings(deleted)
	for _, n := range deleted {
		fmt.Fprintf(s.out, "  - service %s (deleted)\n", n)
	}
	return nil
}

func (m *ConfigMode) Commit(s *Shell) error {
	current, err := config.LoadConfig(m.configPath)
	if err != nil {
		return err
	}

	var next []config.Service
	for _, svc := range current.Services {
		if m.deleted[svc.Name] {
			continue
		}
		if _, ok := m.staged[svc.Name]; ok {
			continue
		}
		next = append(next, svc)
	}
	var stagedNames []string
	for name, svc := range m.staged {
		stagedNames = append(stagedNames, name)
		next = append(next, svc)
	}
	sort.Strings(stagedNames)

	current.Services = next
	if err := config.Validate(current); err != nil {
		return err
	}

	if err := os.MkdirAll(m.configDir, 0755); err != nil {
		return err
	}
	for _, name := range stagedNames {
		fmt.Fprintf(s.out, "Writing %s...\n", filepath.Join(m.configDir, name+".yaml"))
		if err := config.WriteServiceConfig(m.configDir, m.staged[name]); err != nil {
			return err
		}
	}
	var deletedNames []string
	for name := range m.deleted {
		deletedNames = append(deletedNames, name)
	}
	sort.Strings(deletedNames)
	for _, name := range deletedNames {
		_ = os.Remove(filepath.Join(m.configDir, name+".yaml"))
	}

	m.staged = make(map[string]config.Service)
	m.deleted = make(map[string]bool)
	fmt.Fprintln(s.out, "Committed.")
	return nil
}

func (m *ConfigMode) diff() (added []string, updated []string) {
	baseSet := make(map[string]bool)
	for _, svc := range m.base.Services {
		baseSet[svc.Name] = true
	}
	for name := range m.staged {
		if baseSet[name] {
			updated = append(updated, name)
		} else {
			added = append(added, name)
		}
	}
	sort.Strings(added)
	sort.Strings(updated)
	return added, updated
}

