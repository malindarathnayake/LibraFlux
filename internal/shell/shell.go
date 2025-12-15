package shell

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

var ErrExitShell = errors.New("exit shell")

type Mode int

const (
	ModeRoot Mode = iota
	ModeConfig
	ModeService
)

type ShellOptions struct {
	In          io.Reader
	Out         io.Writer
	Err         io.Writer
	ConfigPath  string
	ConfigDir   string
	LockManager *LockManager
	IdleTimeout time.Duration
	Now         func() time.Time
}

type Shell struct {
	in          io.Reader
	out         io.Writer
	err         io.Writer
	configPath  string
	configDir   string
	lockManager *LockManager
	idleTimeout time.Duration
	now         func() time.Time

	mode        Mode
	configMode  *ConfigMode
	serviceMode *ServiceMode
}

func New(opts ShellOptions) (*Shell, error) {
	if opts.Out == nil || opts.Err == nil {
		return nil, errors.New("Out and Err are required")
	}
	if opts.ConfigPath == "" || opts.ConfigDir == "" {
		return nil, errors.New("ConfigPath and ConfigDir are required")
	}
	if opts.LockManager == nil {
		return nil, errors.New("LockManager is required")
	}
	if opts.In == nil {
		opts.In = strings.NewReader("")
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	if opts.IdleTimeout == 0 {
		opts.IdleTimeout = 10 * time.Minute
	}

	return &Shell{
		in:          opts.In,
		out:         opts.Out,
		err:         opts.Err,
		configPath:  opts.ConfigPath,
		configDir:   opts.ConfigDir,
		lockManager: opts.LockManager,
		idleTimeout: opts.IdleTimeout,
		now:         opts.Now,
		mode:        ModeRoot,
	}, nil
}

func (s *Shell) Mode() Mode { return s.mode }

func (s *Shell) Prompt() string {
	switch s.mode {
	case ModeConfig:
		return "lbctl(config)> "
	case ModeService:
		return "lbctl(config-svc)> "
	default:
		return "lbctl> "
	}
}

func (s *Shell) Run(ctx context.Context) error {
	sc := bufio.NewScanner(s.in)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if !sc.Scan() {
			return nil
		}
		line := sc.Text()
		if err := s.ExecuteLine(strings.TrimSpace(line)); err != nil {
			if errors.Is(err, ErrExitShell) {
				return nil
			}
			fmt.Fprintf(s.err, "error: %v\n", err)
		}
	}
}

func (s *Shell) ExecuteLine(line string) error {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}

	if s.mode == ModeConfig || s.mode == ModeService {
		if s.configMode != nil && s.idleTimeout > 0 {
			last := s.configMode.lock.Metadata().LastActivity
			if !last.IsZero() && s.now().UTC().Sub(last) > s.idleTimeout {
				fmt.Fprintf(s.out, "Session idle for %s. Releasing lock...\n", s.idleTimeout.Round(time.Second))
				_ = s.configMode.Abort(s)
				s.leaveConfigureMode()
				return nil
			}
		}
	}

	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return nil
	}
	if tokens[0] == "?" {
		tokens = []string{"help"}
	}

	var err error
	switch s.mode {
	case ModeRoot:
		err = s.handleRoot(tokens)
	case ModeConfig:
		err = s.handleConfig(tokens)
	case ModeService:
		err = s.handleService(tokens)
	default:
		err = fmt.Errorf("unknown mode: %d", s.mode)
	}
	if err != nil {
		return err
	}
	if s.mode == ModeConfig || s.mode == ModeService {
		if s.configMode != nil && s.configMode.lock != nil {
			_ = s.configMode.lock.UpdateActivity()
		}
	}
	return nil
}

func (s *Shell) enterConfigureMode() error {
	if s.configMode != nil {
		return nil
	}
	lock, err := s.lockManager.Acquire(DefaultIdentity())
	if err != nil {
		return err
	}
	cm, err := NewConfigMode(s.configPath, s.configDir, s.idleTimeout, lock)
	if err != nil {
		_ = lock.Release()
		return err
	}
	s.configMode = cm
	s.mode = ModeConfig
	return nil
}

func (s *Shell) leaveConfigureMode() {
	if s.configMode != nil && s.configMode.lock != nil {
		_ = s.configMode.lock.Release()
	}
	s.configMode = nil
	s.serviceMode = nil
	s.mode = ModeRoot
}

func (s *Shell) enterServiceMode(name string) error {
	if s.configMode == nil {
		return errors.New("not in configure mode")
	}
	sm, err := s.configMode.EnterService(name)
	if err != nil {
		return err
	}
	s.serviceMode = sm
	s.mode = ModeService
	return nil
}

func (s *Shell) leaveServiceMode() error {
	if s.configMode == nil || s.serviceMode == nil {
		s.mode = ModeConfig
		s.serviceMode = nil
		return nil
	}
	if err := s.configMode.StageService(s.serviceMode.Service); err != nil {
		return err
	}
	s.serviceMode = nil
	s.mode = ModeConfig
	return nil
}

