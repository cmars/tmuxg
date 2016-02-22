package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/errgo.v1"
	"gopkg.in/yaml.v2"
)

type session struct {
	Name        string            `yaml:"name"`
	SetupScript string            `yaml:"setup-script"`
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

	name, err := locateSession(os.Args[1])
	if err != nil {
		return errgo.Mask(err)
	}

	session, err := newSession(name)
	if err != nil {
		return errgo.Mask(err)
	}

	for k, v := range session.Environment {
		os.Setenv(k, os.ExpandEnv(v))
	}

	log.Println(os.Environ())

	err = session.setupScript()
	if err != nil {
		return errgo.Notef(err, "failed to execute setup script")
	}

	err = session.create()
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

func locateSession(name string) (string, error) {
	if _, err := os.Stat(name); err == nil {
		return name, nil
	} else if !os.IsNotExist(err) {
		return "", errgo.Notef(err, "failed to open session file %q", name)
	}

	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(os.Getenv("HOME"), ".config")
	}

	tmuxgConfigDir := filepath.Join(configDir, "tmuxg")
	err := os.MkdirAll(tmuxgConfigDir, 0644)
	if err != nil {
		return "", errgo.Notef(err, "failed to create config directory %q", tmuxgConfigDir)
	}

	path := filepath.Join(tmuxgConfigDir, name+".yaml")
	if _, err := os.Stat(path); err != nil {
		return "", errgo.Notef(err, "failed to resolve session %q file %q", name, path)
	}
	return path, nil
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
	c := exec.Command("tmux", append([]string{"-L", s.Name}, args...)...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Dir = os.ExpandEnv(s.Cwd)
	log.Printf("%v", c)
	return errgo.Mask(c.Run())
}

func (s *session) setupScript() error {
	if s.SetupScript == "" {
		return nil
	}

	f, err := ioutil.TempFile("", "tmuxg-setup")
	if err != nil {
		return errgo.Notef(err, "failed to create temporary file for script")
	}
	defer os.Remove(f.Name())
	defer f.Close()

	_, err = fmt.Fprintf(f, "%s", strings.TrimSpace(s.SetupScript))
	if err != nil {
		return errgo.Notef(err, "failed to write temporary script file")
	}
	err = f.Close()
	if err != nil {
		return errgo.Notef(err, "error closing temporary script file")
	}
	err = os.Chmod(f.Name(), 0700)
	if err != nil {
		return errgo.Notef(err, "error setting temporary script executable")
	}

	c := exec.Command(f.Name())
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
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

	var keys []string
	for k, v := range s.Environment {
		err = s.tmux("set-environment", k, os.ExpandEnv(v))
		if err != nil {
			return errgo.Notef(err, "failed to set environment variable %q", k)
		}
		keys = append(keys, k)
	}

	return nil
}

func (s *session) createWindow(i int, w *window) error {
	cwd := w.Cwd
	if cwd == "" {
		cwd = s.Cwd
	}
	cwd = os.ExpandEnv(cwd)
	err := s.tmux("new-window", "-d", "-t", fmt.Sprintf("%s:%d", s.Name, i),
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
