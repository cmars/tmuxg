package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"

	"gopkg.in/errgo.v1"
	"gopkg.in/yaml.v2"
)

type session struct {
	Name        string            `yaml:"name"`
	Environment map[string]string `yaml:"environment"`
	Cwd         string            `yaml:"cwd"`
	Windows     []window          `yaml:"windows"`
	Focus       string            `yaml:"focus"`
}

type window struct {
	Name       string   `yaml:"name"`
	Command    string   `yaml:"command"`
	Cwd        string   `yaml:"cwd"`
	Keystrokes []string `yaml:"keystrokes"`
}

func die(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, errgo.Details(err))
		os.Exit(1)
	}
}

func main() {
	die(run())
}

func run() error {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: %s <session.yaml file>", os.Args[0])
		return errgo.New("missing session file argument")
	}

	session, err := newSession(os.Args[1])
	if err != nil {
		return errgo.Mask(err)
	}

	for k, v := range session.Environment {
		os.Setenv(k, os.ExpandEnv(v))
	}

	err = session.create()
	if err != nil {
		return errgo.Mask(err)
	}

	err = session.updateEnvironment()
	if err != nil {
		return errgo.Mask(err)
	}

	var focus int
	for i, window := range session.Windows {
		if i > 0 {
			err = session.createWindow(i, &window)
			if err != nil {
				return errgo.Mask(err)
			}
		}

		err = session.sendKeys(&window)
		if err != nil {
			return errgo.Mask(err)
		}

		if session.Focus == window.Name {
			focus = i
		}
	}
	err = session.focus(focus)
	if err != nil {
		return errgo.Mask(err)
	}

	err = session.tmux("attach", "-t", session.Name)
	return errgo.Mask(err)
}

func newSession(path string) (*session, error) {
	var s session

	contents, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errgo.Notef(err, "failed to read session file")
	}
	err = yaml.Unmarshal(contents, &s)
	if err != nil {
		return nil, errgo.Notef(err, "failed to parse session file")
	}
	return &s, nil
}

func (s *session) tmux(args ...string) error {
	c := exec.Command("tmux", args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = os.ExpandEnv(s.Cwd)
	log.Printf("%v", c)
	return errgo.Mask(c.Run())
}

func (s *session) create() error {
	if len(s.Windows) == 0 {
		return errgo.New("no windows configured for this session!")
	}
	err := s.tmux("new-session", "-d", "-s", s.Name,
		os.ExpandEnv(s.Windows[0].Command))
	if err != nil {
		return errgo.Notef(err, "failed to start tmux session")
	}
	return nil
}

func (s *session) updateEnvironment() error {
	for k, v := range s.Environment {
		err := s.tmux("set-environment", "-t", s.Name, k, os.ExpandEnv(v))
		if err != nil {
			return errgo.Notef(err, "failed to set session environment variables")
		}
	}
	return nil
}

func (s *session) createWindow(i int, w *window) error {
	cwd := w.Cwd
	if cwd == "" {
		cwd = s.Cwd
	}
	cwd = os.ExpandEnv(cwd)
	err := s.tmux("new-window", "-t", fmt.Sprintf("%s:%d", s.Name, i),
		"-c", cwd, os.ExpandEnv(w.Command))
	if err != nil {
		return errgo.Notef(err, "failed to create window %q", w.Name)
	}
	return nil
}

func (s *session) sendKeys(w *window) error {
	if len(w.Keystrokes) > 0 {
		err := s.tmux(append([]string{"send-keys"}, w.Keystrokes...)...)
		if err != nil {
			return errgo.Notef(err, "failed to send keystrokes to window %q", w.Name)
		}
	}
	return nil
}

func (s *session) focus(i int) error {
	err := s.tmux("select-window", "-t", fmt.Sprintf("%s:%d", s.Name, i))
	if err != nil {
		return errgo.Notef(err, "failed to set window focus")
	}
	return nil
}
