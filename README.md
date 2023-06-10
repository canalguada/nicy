# Nicy (WIP)
## About
Set the execution environment and configure the available resources for spawned and running processes.

## Why
Low-end hardware quickly turns nasty, running up to date "resource hungry" programs. But controlling the resources that they request help keeping that hardware serviceable.

I write nicy because, sometimes, I need to control the available resources per program according to some more context, in other words adding options in a command line, not editing a configuration file as a privileged user. Something that [Ananicy](https://github.com/Nefelim4ag/Ananicy), an auto nice daemon, with community rules support, that relies too on the Linux Control Groups ([Cgroups](https://en.wikipedia.org/wiki/Cgroups)), does not provide.

nicy was first implemented as a [shell script](https://github.com/canalguada/nicy/tree/sh#start-of-content), but now that it is implemented in Go langage, nicy can gain capabilities at install and run time. User does not need superuser privileges in order to run most of commands in an non-interactive context.

## Description
nicy relies on existing system utilities and can be used to ease the control upon the execution environment of the managed processes and to configure the resources available to them: with renice(1), can alter their scheduling priority; with chrt(1), can set their real-time scheduling attributes and with ionice(1) their I/O scheduling class and priority; with choom(1), can adjust their Out-Of-Memory killer score setting.

nicy can also create and start a transient systemd scope unit and either run the specified command and its spawned processes in it with systemd-run(1), or move yet running processes inside it.

When used to launch commands, nicy can also automatically change some environment variables and add command arguments.

nicy manages the processes applying them generic or specific presets stored in YAML format.

## Usage
```
$ nicy run --dry-run apt upgrade
nicy: run: dry-run: /usr/bin/renice -n 19 -p 15325
nicy: run: dry-run: /usr/bin/chrt --idle -a -p 0 15325
nicy: run: dry-run: /usr/bin/ionice -c 3 -p 15325
nicy: run: dry-run: /usr/bin/systemctl --user start nicy-cpu33.slice
nicy: run: dry-run: /usr/bin/systemctl --user --runtime set-property nicy-cpu33.slice CPUQuota=33%
nicy: run: dry-run: /usr/bin/systemd-run --user -G -d --no-ask-password --quiet --scope --unit=apt-cpu33-15325 --slice=nicy-cpu33.slice /usr/bin/apt upgrade

```
```
$ NICY_SHELL=/usr/bin/zsh nicy show nvim
#!/usr/bin/zsh
(( $UID )) && SUDO=$SUDO || unset SUDO
$SUDO renice -n -3 -p $$ >/dev/null
(( $UID )) && user_or_system=--user || user_or_system=--system
systemctl ${user_or_system} start nicy-cpu80.slice >/dev/null
systemctl ${user_or_system} --runtime set-property nicy-cpu80.slice CPUQuota=80% >/dev/null
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet --scope --unit=nvim-cpu80-$$ --slice=nicy-cpu80.slice -E SHELL=/bin/bash /usr/bin/nvim --listen /tmp/nvimsocket "$@"
```
```
# nicy control --dry-run --system
nicy: control: dry-run: cupsd[465]: cgroup:cups.service pids:[465]
nicy: control: dry-run: cupsd[465]: /usr/bin/renice -n 19 -g 465
nicy: control: dry-run: cupsd[465]: /usr/bin/ionice -c 3 -P 465
nicy: control: dry-run: cupsd[465]: /usr/bin/chrt --idle -a -p 0 465
nicy: control: dry-run: cups-browsed[538]: cgroup:cups-browsed.service pids:[538]
nicy: control: dry-run: cups-browsed[538]: /usr/bin/renice -n 19 -g 538
nicy: control: dry-run: cups-browsed[538]: /usr/bin/ionice -c 3 -P 538
nicy: control: dry-run: cups-browsed[538]: /usr/bin/chrt --idle -a -p 0 538
nicy: control: dry-run: apache2[916]: cgroup:apache2.service pids:[916 917 919 920]
nicy: control: dry-run: apache2[916]: /usr/bin/renice -n 19 -g 916
nicy: control: dry-run: apache2[916]: /usr/bin/ionice -c 2 -n 7 -P 916

```
```
$ nicy control --dry-run --user
nicy: control: dry-run: pulseaudio[1299]: cgroup:pulseaudio.service pids:[1299]
nicy: control: dry-run: pulseaudio[1299]: /usr/bin/ionice -c 1 -P 1299
nicy: control: dry-run: pulseaudio[1299]: /usr/bin/chrt --rr -a -p 1 1299
nicy: control: dry-run: nvim[2270]: cgroup:nvim-2270.scope pids:[2270]
nicy: control: dry-run: nvim[2270]: /usr/bin/systemctl --user start nicy-cpu33.slice
nicy: control: dry-run: nvim[2270]: /usr/bin/systemctl --user --runtime set-property nicy-cpu33.slice CPUQuota=33%
nicy: control: dry-run: nvim[2270]: /usr/bin/busctl call --quiet --user org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager StartTransientUnit ssa(sv)a(sa(sv)) nvim-2270.scope fail 2 PIDs au 1 2270 Slice s nicy-cpu33.slice 0
nicy: control: dry-run: cmus[2274]: cgroup:session-1.scope pids:[2274]
nicy: control: dry-run: cmus[2274]: /usr/bin/renice -n -3 -g 2274
nicy: control: dry-run: cmus[2274]: /usr/bin/ionice -c 1 -P 2274
```

## Requirements
* [systemd](https://systemd.io/) and [systemd-run](https://www.freedesktop.org/software/systemd/man/systemd-run.html)

renice, chrt, ionice and choom system utilities are provided by essential packages, at least on Debian (bsdutils and util-linux).

Most of cgroup settings are supported only with the unified control group hierarchy, the new version of kernel control group interface. See [Control Groups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).

## Installation
### From source
Build:
```
$ make
```
Install in /usr/local:
```
$ sudo make install
```
Install in /usr:
```
$ sudo make prefix=/usr install
```
### Building Debian package
Require [debmake](https://manpages.debian.org/buster/debmake/debmake.1.en.html) and [debuild](https://manpages.debian.org/buster-backports/devscripts/debuild.1.fr.html):
```
$ make deb
```

## Configuration
See [CONFIGURATION.md](https://github.com/canalguada/nicy/blob/master/CONFIGURATION.md) file.

## TODO
* Implement `install` missing subcommand.
* Fix Debian and Archlinux packaging.
* Provide tests and document code.

