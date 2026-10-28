package main

import (
	"errors"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
	"golang.org/x/term"
)

// The picker owns its raw-mode interval and uses readiness polling rather than
// leaving a blocked input goroutine that could steal the monitor's keystrokes.
func chooseRepositories(paths []string, out io.Writer) ([]string, error) {
	if !term.IsTerminal(int(os.Stdin.Fd())) || !term.IsTerminal(int(os.Stdout.Fd())) {
		return nil, errors.New("repository selection requires an interactive terminal; open the application inside a Git repository")
	}
	old, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}
	defer term.Restore(int(os.Stdin.Fd()), old)
	if _, err = io.WriteString(out, "\x1b[?1049h\x1b[?7l\x1b[?25l\x1b[?1003h\x1b[?1006h\x1b[2J"); err != nil {
		return nil, err
	}
	defer io.WriteString(out, "\x1b[?1006l\x1b[?1003l\x1b[?25h\x1b[?7h\x1b[0m\x1b[?1049l")
	signals := make(chan os.Signal, 2)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signals)
	p := newRepositoryPicker(paths)
	fd := int(os.Stdin.Fd())
	var fds unix.FdSet
	if fd < 0 || fd >= len(fds.Bits)*unix.NFDBITS {
		return nil, errors.New("terminal input descriptor exceeds picker polling capacity")
	}
	for !p.done && !p.cancelled {
		select {
		case <-signals:
			return nil, nil
		default:
		}
		w, h, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil {
			return nil, err
		}
		if _, err = io.WriteString(out, p.render(w, h)); err != nil {
			return nil, err
		}
		fds.Zero()
		fds.Set(fd)
		timeout := unix.NsecToTimeval((100 * time.Millisecond).Nanoseconds())
		n, err := unix.Select(fd+1, &fds, nil, nil, &timeout)
		if err == unix.EINTR {
			continue
		}
		if err != nil {
			return nil, err
		}
		if n > 0 {
			buffer := make([]byte, 256)
			n, err = os.Stdin.Read(buffer)
			if err != nil {
				return nil, err
			}
			if n == 0 {
				return nil, nil
			}
			p.input(buffer[:n])
		}
	}
	if p.cancelled {
		return nil, nil
	}
	return p.selectedPaths(), nil
}
