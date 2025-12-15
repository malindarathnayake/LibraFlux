package shell

import (
	"errors"
	"fmt"
	"strings"
)

func (s *Shell) handleRoot(tokens []string) error {
	switch strings.ToLower(tokens[0]) {
	case "help":
		return PrintHelp(s, ModeRoot)
	case "exit":
		return ErrExitShell
	case "configure":
		if err := s.enterConfigureMode(); err != nil {
			return err
		}
		if len(tokens) > 1 {
			return s.handleConfig(tokens[1:])
		}
		return nil
	case "lock":
		if len(tokens) < 2 {
			return PrintHelp(s, ModeRoot)
		}
		switch strings.ToLower(tokens[1]) {
		case "status":
			meta, err := s.lockManager.Status()
			if err != nil {
				return err
			}
			if meta == nil {
				fmt.Fprintln(s.out, "No configuration lock held.")
				return nil
			}
			fmt.Fprintf(s.out, "Configuration locked by %s@%s (PID %d)\n", meta.User, meta.Host, meta.PID)
			return nil
		case "break":
			force := len(tokens) >= 3 && tokens[2] == "--force"
			return s.lockManager.Break(force)
		default:
			return fmt.Errorf("unknown lock command: %s", tokens[1])
		}
	case "show":
		fmt.Fprintln(s.out, "show: not implemented (daemon integration in Phase 7)")
		return nil
	case "doctor":
		fmt.Fprintln(s.out, "doctor: not implemented (Phase 7)")
		return nil
	case "reload":
		fmt.Fprintln(s.out, "reload: not implemented (Phase 7)")
		return nil
	default:
		return fmt.Errorf("unknown command: %s", tokens[0])
	}
}

func (s *Shell) handleConfig(tokens []string) error {
	if s.configMode == nil {
		return errors.New("not in configure mode")
	}
	switch strings.ToLower(tokens[0]) {
	case "help":
		return PrintHelp(s, ModeConfig)
	case "exit":
		_ = s.configMode.Abort(s)
		s.leaveConfigureMode()
		return nil
	case "abort":
		_ = s.configMode.Abort(s)
		return nil
	case "commit":
		return s.configMode.Commit(s)
	case "show":
		return s.configMode.ShowPending(s)
	case "service":
		if len(tokens) < 2 {
			return errors.New("usage: service <name>")
		}
		return s.enterServiceMode(tokens[1])
	case "delete":
		if len(tokens) < 2 {
			return errors.New("usage: delete <service>")
		}
		return s.configMode.DeleteService(tokens[1])
	default:
		return fmt.Errorf("unknown configure command: %s", tokens[0])
	}
}

func (s *Shell) handleService(tokens []string) error {
	if s.serviceMode == nil || s.configMode == nil {
		return errors.New("not in service mode")
	}

	switch strings.ToLower(tokens[0]) {
	case "help":
		return PrintHelp(s, ModeService)
	case "exit":
		return s.leaveServiceMode()
	default:
		return s.serviceMode.Handle(s, tokens)
	}
}

