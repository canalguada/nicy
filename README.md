# Nicy
## About
Set the execution environment and configure the resources that spawned and running processes are allowed to share.

## Why
My legacy low-end hardware quickly turns nasty, running hot then shutting down, when launching too many or modern "resource hungry" softwares. But controlling the resources that some of them request help that hardware to be serviceable again.

I used to install [Ananicy](https://github.com/Nefelim4ag/Ananicy), an auto nice daemon, with community rules support, that relies too on the Linux Control Groups ([Cgroups](https://en.wikipedia.org/wiki/Cgroups)).

I write nicy because I need to control the available resources per program according to some more context, in other words adding options in a command line, not editing a configuration file as a privileged user.

## Description
nicy is a script that relies on existing system utilities and can be used to ease the control upon the execution environment of the managed processes and to configure the resources available to them: with renice(1), can alter their scheduling priority; with chrt(1), can set their real-time scheduling attributes and with ionice(1) their I/O scheduling class and priority; with choom(1), can adjust their Out-Of-Memory killer score setting.

nicy can also create and start a transient systemd scope unit and either run the specified command and its spawned processes in it with systemd-run(1), or move yet running processes inside it.

When used to launch commands, nicy can also automatically change some environment variables and add command arguments.

nicy manages the processes applying them generic or specific presets stored in JSON[1] format. The data is accessed either with jq(1), a lightweight and flexible command-line JSON processor, or with another implementation of this program.

## Usage
```
$ nicy run --dry-run go get github.com/itchyny/gojq/cmd/gojq
nicy: run: systemctl --user start nicy-cpu16.slice
nicy: run: systemctl --user --runtime set-property nicy-cpu16.slice CPUQuota=16%
nicy: run: exec systemd-run --user -G -d --no-ask-password --quiet --unit=go-20156 --scope --slice=nicy-cpu16.slice --nice=19 chrt --idle 0 ionice -c 3 /usr/bin/go get github.com/itchyny/gojq/cmd/gojq
```
```
$ nicy show nvim-qt
#!/bin/sh
[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system=
[ $(id -u) -ne 0 ] && SUDO=sudo || SUDO=
$SUDO renice -n -3 -p $$ >/dev/null 2>&1
systemctl ${user_or_system} start nicy-cpu66.slice >/dev/null 2>&1
systemctl ${user_or_system} --runtime set-property nicy-cpu66.slice CPUQuota=66% >/dev/null 2>&1
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet --unit=nvim-qt-$$ --scope --slice=nicy-cpu66.slice -E 'SHELL=/bin/bash -l' /usr/bin/nvim-qt --nofork --nvim /usr/bin/nvim "$@"
```
```
# nicy manage --dry-run --system
nicy: manage: adjusting comm:cupsd pgrp:460 cgroup:cups.service pids:460
nicy: manage: renice -n 19 -g 460
nicy: manage: ionice -c 3 -P 460
nicy: manage: chrt --idle -a -p 0 460
nicy: manage: systemctl --runtime set-property cups.service CPUQuota=16%
nicy: manage: adjusting comm:lightdm pgrp:540 cgroup:lightdm.service pids:540
nicy: manage: ionice -c 1 -n 4 -P 540
nicy: manage: adjusting comm:cups-browsed pgrp:550 cgroup:cups-browsed.service pids:550
nicy: manage: renice -n 19 -g 550
nicy: manage: ionice -c 3 -P 550
nicy: manage: chrt --idle -a -p 0 550
nicy: manage: systemctl --runtime set-property cups-browsed.service CPUQuota=16%
nicy: manage: adjusting comm:Xorg pgrp:580 cgroup:lightdm.service pids:580
nicy: manage: renice -n -10 -g 580
nicy: manage: ionice -c 1 -n 1 -P 580
nicy: manage: adjusting comm:apache2 pgrp:974 cgroup:apache2.service pids:974
nicy: manage: renice -n 19 -g 974
nicy: manage: ionice -c 2 -n 7 -P 974
nicy: manage: systemctl --runtime set-property apache2.service CPUQuota=66%
```
```
$ nicy --help
nicy version 0.1.2

Set the execution environment and configure the resources that spawned
and running processes are allowed to share.

Usage:
  nicy run [OPTION]... COMMAND [ARGUMENT]...
    Run the COMMAND in a pre-set execution environment.
  nicy show [OPTION]... COMMAND
    Show the effective script for these COMMAND.
  nicy list [OPTION] CATEGORY
    List the objects from this CATEGORY, removing all duplicates.
  nicy install [OPTION]...
    Install scripts.
  nicy rebuild [OPTION]
    Rebuild the json cache and exit.
  nicy manage [OPTION]...
    Manage the running processes, applying presets.

Common options:
  -h, --help           display help and exit
  -V, --version        show version and exit

Run and show options:
  -q, --quiet          suppress additional output
  -v, --verbose        display which command is launched
  -p, --preset=PRESET  apply this PRESET
  -d, --default        like '--preset=default'
  -z, --cgroup-only    like '--preset=cgroup-only'
  -c, --cgroup=CGROUP  run as part of this CGROUP
      --cpu<QUOTA>     like '--cgroup=cpu<QUOTA>'
  -m, --managed        always run inside its own scope
  -u, --force-cgroup   run inside a cgroup matching properties

Manage options:
      --user           only inside calling user slice
      --system         only inside system slice
      --global         inside any user slice
      --all            inside either system or users slice

Run and manage option:
  -n, --dry-run        display commands but do not run them

List options:
  -f, --from=CONFDIR   list only objects from CONFDIR directory

Install options:
      --shell=SHELL    generate script for SHELL
      --path=DESTDIR   install inside DESTDIR

Rebuild options:
      --force          ignore existing files in cache

  The PRESET  argument can be:  'auto' to use  some specific rule  for the
  command, if available;  'cgroup-only' to use only  the cgroup properties
  of that rule, if any; 'default'  to use this special fallback preset; or
  any other generic  type. The CGROUP argument can be  a cgroup defined in
  configuration files. The QUOTA argument can be an integer ranging from 1
  to  99 that  represents  a percentage  relative to  the  total CPU  time
  available on all cores.

  The processes  are managed per  process group,  when a specific  rule is
  available for the process group leader. The --system, --global and --all
  options require root credentials.

  The CATEGORY argument can be 'rules', 'types' or 'cgroups', matching the
  extensions of configuration  files. The CONFDIR argument can  be one out
  of NICY_CONF  directories. When filtering  per CONFDIR, no  duplicate is
  removed taking into account the priority between directories.

  The SHELL argument must be a path  to a supported shell (sh, dash, bash,
  zsh).  Default value  is  /bin/sh.  The scripts  are  installed, when  a
  specific  rule is  available,  in  DESTDIR if  given.  Default value  is
  $HOME/bin/nicy, or /usr/local/bin/nicy for superuser.

```

## Requirements
* [systemd](https://systemd.io/) and [systemd-run](https://www.freedesktop.org/software/systemd/man/systemd-run.html)
* [jq](https://stedolan.github.io/jq/), a lightweight and flexible command-line JSON processor

The other system utilities (renice, chrt, ionice and choom) are provided by essential packages, at least on Debian (bsdutils and util-linux).

Most of cgroup settings are supported only with the unified control group hierarchy, the new version of kernel control group interface. See [Control Groups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).

## Installation
### From source
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
- Switch to python (the easy way) before reaching 2k LOC.

