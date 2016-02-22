# tmuxg

tmuxg automates tmux session setup from a model.

The model defines a set of windows that run various commands, with
pre-defined current working directory and environment variables.

Environment variables may be isolated to a session. That's important to me, as
I often need to work with multiple projects that have different `$GOPATH`
settings.

tmuxg projects are declared with YAML files.

tmuxg is inspired by tmuxp but tmuxp didn't quite do things the way I wanted,
and did a bunch of things I didn't need it to do.

# Build & install

    $ go get github.com/cmars/tmuxg

# Example

Here's an example that sets up several windows:

```
name: myproject
environment:
  GOPATH: ${HOME}/myproject/gopath
cwd: ${GOPATH}/src/github.com/cmars/myproject
windows:
  - name: editor
    command: vim
    keystrokes:
      - \n
  - name: shell
  - name: top
    command: top
  - name: local lxc containers
    command: watch lxc list
  - name: staging log
    command: ssh ubuntu@staging "lxc exec appserver -- tail -f /var/log/syslog"
focus: editor
```

Environment variables may be used in `cwd` and `command` values, including
variables declared in the `environment` section.

The 'keystrokes' are taken literally, same format as the `tmux send-keys`
command. In the example above, `<Backslash>` is my vim leader-key,
`<Leader>n` opens NERDTree. Use `C-m` to literally send an `<Enter>` press.

# TODO

tmuxg meets most of my minimal needs.

Some features I _might_ add:

- Setting terminal window title from session.
- Setting tmux window titles, auto-naming.
- Loading the YAML files by basename from a well-known path (`~/.config/tmuxg or something).
- Controlling the status line.
- Attaching to an already-running session by the same name.
- Support for declaring panes in windows.
- Some tests.
