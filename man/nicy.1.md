---
title: nicy
section: 1
header: General Commands Manual
footer: nicy %version%
date: June 2023
---

# NAME

nicy - set the execution environment and configure the resources that spawned
and running processes are allowed to share

# SYNOPSIS

`nicy` `run` [`-q`|`-v`] [`-nmu`] [`-p` *preset*|`-d`|`-z`]
[`-c` *cgroup*|`--cpu<quota>`] *COMMAND* [*ARGUMENT*]...

`nicy` `show` [`-q`|`-v`] [`-mu`] [`-p` *preset*|`-d`|`-z`]
[`-c` *cgroup*|`--cpu<quota>`] *COMMAND*

`nicy` `list` [`-n`] [`-f` *origin*] *CATEGORY*

`nicy` `build` [`-d`] [`-f`]

`nicy` `set` [`-n`] [`-u`|`-g`|`-s`|`-a`]

`nicy` `control` [`-n`] [`-u`|`-g`|`-s`|`-a`] [`-t` *SECONDS*]

`nicy` `dump` [`-u`|`-g`|`-s`|`-a`] [`-r`|`-j`|'-n`] [`-m`]

`nicy` `install` [`-r`] [`--shell` *SHELL*] [`--dest` *DESTDIR*]

# DESCRIPTION

`nicy` relies on existing system utilities  and  can  be
used  to ease the control upon the execution environment of the managed
processes and to  configure  the  resources  available  to  them:  with
`renice`(1),  can  alter their scheduling priority; with `chrt`(1), can set
their real-time scheduling attributes  and  with  `ionice`(1)  their  I/O
scheduling  class and priority; with `choom`(1), can adjust their Out-Of-
Memory killer score setting.

`nicy` can also create and start a transient systemd scope unit  and  ei‐
ther  run  the  specified  command and its spawned processes in it with
`systemd-run`(1), or move yet running processes inside it.

When used to launch commands, `nicy` can also automatically  change  some
environment variables and add command arguments.

`nicy`  manages  the  processes applying them generic or specific presets
stored in YAML[1] format.

Unless the *-h*, *--help*, *-V* or *--version* option  is  given,  one  of  the
builtin commands below must be present.

# BUILTIN COMMANDS

`run` [`option`]... *COMMAND* [*ARGUMENT*]...
: Run the *COMMAND* and its *ARGUMENT*(s) in a pre-set execution environment.

`show` [`option`]... *COMMAND*
: Show the effective script for this *COMMAND*.

`list` [`option`]... *CATEGORY*
: List the objects from given *CATEGORY*, removing all duplicates. The argument
can either be rules, profiles or cgroups.

`build` [`option`]...
: Build the yaml cache and exit.

`set` [`option`]...
: Set once the running processes, applying pre-set rules, if any.

`control` [`option`]...
: Control the running processes, applying pre-set rules, if any.

`dump` [`option`]...
: Dump information about the running processes.

`install` [`option`]...
: Install a shell script for each rule matching a command in PATH.

# OPTIONS

## Global options:

`-h`, `--help`
: Display help for the program or its subcommand and exit.

`-V`, `-version`
: Show the program version and exit.

`-q`, `--quiet`
: Suppress additional output.

`-v`, `--verbose`
: Be more verbose.

The following options are only available with the specified commands.

## Run and show options:

`-p` *preset*, `--preset=`*preset*
: Apply the specified preset which can be
`auto` to use some specific rule for the command, if available;
`cgroup-only` to use only the cgroup properties of that rule;
`default` to use this special fallback preset;
or any other generic profile. See CONFIGURATION.

The implied default is auto. Fallback preset is used when a rule is required
but none is available.

`-d`, `--default`
: Like `--preset=`*default*.

`-z`, `--cgroup-only`
: Like `--preset=`*cgroup-only*.

`-c` *cgroup*, `--cgroup=`*cgroup*
: Run the command as part of the *nicy-<cgroup>.slice* whose properties have
been set at runtime to match the specified cgroup entry from configuration file.
See CONFIGURATION.

`--cpu<quota>`
: Like `--cgroup=`*cpu<quota>*. The *quota* argument can be an integer ranging
from *1 to 99* that represents a percentage relative to the total CPU time
available *on all cores*.

`-m`, `--managed`
: Always run the command inside its own scope.

`-u`, `--force-cgroup`
: Run the command inside a cgroup defined in the configuration files, if any,
matching at best the required properties.

## Dump, set and control options:

`-u`, `--user`
: Set or control only the processes running inside the calling user slice.

`-s`, `--system`
: Set or control only the processes running inside the system slice.

`-g`, `--global`
: Set or control the processes running inside any user slice.

`-a`, `--all`
: Set or control all the running processes.

The processes are managed per process group, when a specific rule is available for
the process group leader. The implied default option is `--user`. The `--system`,
`--global` and `--all` options require root credentials.

## Dump options:

`-r`, `--raw`
: Raw format.

`-j`, `--json`
: JSON format.

`-n`, `--values`
: Nicy values format.

`-m`, `--manageable`
: Show only manageable processes.

## Run, set and control options:

`-n`, `--dry-run`
: Perform a simulation but do not actually run anything. Print out a series of
lines, each representing a command.

## List options:

`-f` *origin*, `--from=`*origin*
: List only objects from given *origin*. Must be one out of `user`, `site`,
`vendor` - that qualify a preset supplied through one out of preconfigured path
- or `other` for all other presets. When filtering per origin, no duplicate is
removed taking into account the precedence between directories.

`-n`, `--no-header`
: Do not print headers.

## Build options:

`-f`, `--force`
: Ignore old cache or forcefully build new.

`-d`, `--dump`
: Dump to stdout without saving.

## Install options:

`--shell=`*shell*
: Generate and install scripts for specified shell. Must be a path to a
supported shell (*sh*, *dash*, *bash* and *zsh*). Default value is */bin/sh*.

`--path=`*destdir*
: Install the scripts in *destdir*. Default value is *$HOME/bin/nicy*, or
*/usr/local/bin/nicy* for system user.

`-r`, `--run`
: Use run command when generating the scripts.

# EXAMPLES

## Example 1. Managing the running processes of the current user.

`nicy` can scan the running processes looking for properties to apply in
order to match existing specific rules. For instance, after launching
*pcmanfm-qt* file manager and using the *list* builtin command to check
existing rule and profile, `nicy` can adjust the niceness and I/O
scheduling class properties, then move any process in the *pcmanfm-qt*
process group to its proper scope, with `set` builtin command.

```sh
$ /usr/bin/pcmanfm-qt &>/dev/null &
[1] 2686

$ cat /proc/2686/cgroup
0::/user.slice/user-1001.slice/session-2.scope

$ nicy list rules | grep pcmanfm-qt
pcmanfm-qt    user    {"profile":"File-Manager"}

$ nicy list profiles | grep File-Manager
File-Manager    user    {"nice":-3,"ioclass":"best-effort",
    "cgroup":"cpu80"}

$ nicy set --user
nicy: set: pcmanfm-qt[2686]: cgroup:session-3.scope pids:[2686]
nicy: set: pcmanfm-qt[2686]: sudo renice -n -3 -g 2686
nicy: set: pcmanfm-qt[2686]: ionice -c 2 -P 2686
nicy: set: pcmanfm-qt[2686]: systemctl --user start nicy-cpu33.slice
nicy: set: pcmanfm-qt[2686]: systemctl --user --runtime
    set-property nicy-cpu33.slice CPUQuota=33%
nicy: set: pcmanfm-qt[2686]: busctl call --quiet --user
    org.freedesktop.systemd1
    /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager
    StartTransientUnit ssa(sv)a(sa(sv)) pcmanfm-qt-2686.scope
    fail 2 PIDs au 1 2686 Slice s nicy-cpu33.slice 0

$ systemd-cgls /user.slice/user-1001.slice/user@1001.service/nicy.slice
Control group /user.slice/user-1001.slice/user@1001.service/nicy.slice:
└─nicy-cpu33.slice
  └─pcmanfm-qt-2686.scope
    └─2686 /usr/bin/pcmanfm-qt
```

`nicy` can adjust the properties of almost any process, whether it runs
in user or in system slice.

## Example 2. Setting the processes in system slice as superuser

```sh
# nicy set --dry-run --system
nicy: set: dry-run: smbd[1217]: cgroup:smbd.service pids:[1217]
nicy: set: dry-run: smbd[1217]: /usr/bin/renice -n -10 -g 1217
nicy: set: dry-run: smbd[1217]: /usr/bin/ionice -c 1 -P 1217
nicy: set: dry-run: cupsd[763]: cgroup:cups.service pids:[763]
nicy: set: dry-run: cupsd[763]: /usr/bin/renice -n 19 -g 763
nicy: set: dry-run: cupsd[763]: /usr/bin/ionice -c 3 -P 763
nicy: set: dry-run: cupsd[763]: /usr/bin/chrt --idle -a -p 0 763
```

## Example 3. Setting the processes of all users as superuser.

```sh
# nicy set --global
nicy: set: lightdm[1310]: cgroup:session-3.scope pids:[1310]
nicy: set: lightdm[1310]: ionice -c 1 -n 4 -P 541
nicy: set: pulseaudio[1467]: cgroup:pulseaudio.service pids:[1467]
nicy: set: pulseaudio[1467]: ionice -c 1 -P 1467
nicy: set: pulseaudio[1467]: chrt --rr -a -p 1 1467
nicy: set: i3[1485]: cgroup:session-3.scope pids:[1485]
nicy: set: i3[1485]: renice -n -10 -g 1485
nicy: set: i3[1485]: ionice -c 1 -P 1485
nicy: set: mopidy[3032]: cgroup:mopidy.service pids:[3032]
nicy: set: mopidy[3032]: ionice -c 1 -P 3032
nicy: set: pcmanfm-qt[9064]: cgroup:session-3.scope pids:[9064]
nicy: set: pcmanfm-qt[9064]: renice -n -3 -g 9064
nicy: set: pcmanfm-qt[9064]: runuser -u canalguada -- env
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus
    XDG_RUNTIME_DIR=/run/user/1001
    systemctl --user start nicy-cpu33.slice
nicy: set: pcmanfm-qt[9064]: runuser -u canalguada -- env
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus
    XDG_RUNTIME_DIR=/run/user/1001 systemctl --user --runtime
    set-property nicy-cpu33.slice CPUQuota=33%
nicy: set: pcmanfm-qt[9064]: runuser -u canalguada -- env
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus
    XDG_RUNTIME_DIR=/run/user/1001
    busctl call --quiet --user
    org.freedesktop.systemd1 /org/freedesktop/systemd1
    org.freedesktop.systemd1.Manager StartTransientUnit
    'ssa(sv)a(sa(sv))' pcmanfm-qt-9064.scope fail 2 PIDs
    au 1 9064 Slice s nicy-cpu33.slice 0
```

## Example 4. Installing scripts that replace the nicy commands.

After adding a new rule and using the `build` builtin command to
refresh the cache, with the configuration above,
`nicy run other-browser` roughly executes the script below,
generated with the `show` builtin command:

```sh
$ nicy build --force
Writing "/run/user/1001/nicy/cache.yaml" cache file... Done.

$ nicy show other-browser
#!/bin/sh
[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system=
[ $(id -u) -ne 0 ] && SUDO=sudo || SUDO=
$SUDO renice -n -3 -p $$ >/dev/null 2>&1
systemctl ${user_or_system} start nicy-cpu66.slice >/dev/null 2>&1
systemctl ${user_or_system} --runtime set-property
        nicy-cpu66.slice CPUQuota=66% >/dev/null 2>&1
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet
        --unit=other-browser-$$ --scope --slice=nicy-cpu66.slice
        -p MemoryHigh=60% -p MemoryMax=75% choom -n 1000
        -- /usr/bin/other-browser "$@"
```

Copying this script somewhere at the beginning of PATH turns it
available as a nicy replacement for the `other-browser` command.

With `install` builtin command, if matching an existing command,
`nicy` generates *$NICY_SHELL* scripts for all the specific *rules*
and installs them in *$NICY_SCRIPTS_LOCATION*.
See ENVIRONMENT below.

Long lines are formatted in this section to fit the manpage width.

# FILES

`nicy` reads the following configuration files from *$XDG_CONFIG_HOME* (when
not run by superuser), */usr/local/etc*, and */etc* per order of precedence.

*nicy/config.yaml*
: Configuration file. See `nicy`(5).

# ENVIRONMENT

nicy checks for the existence of these variables in environment.

*XDG_CONFIG_HOME*
: Base directory relative to which user specific configuration files should be
stored. If either not set or empty, defaults to *$HOME/.config*.

*NICY_VERBOSE*
: Be verbose and show every launched command. Defaults to *false*.

*NICY_SHELL*
: Shell used when generating the scripts. Defaults to */bin/bash*.

*NICY_SUDO*
: Command used when the root-credentials are  required. Defaults to `sudo`(8).

*NICY_SCRIPTS_LOCATION*
: Path to directory where the scripts are installed. Defaults to
*$HOME/bin/nicy* or */usr/local/bin/nicy* for superuser.

# EXIT STATUS

* `0`      No problems occurred.
* `1`      Generic error code.
* `2`      Generic parse error with command-line options.
* `3`      Directory mismatch. Not in configuration list.
* `4`      Not a directory.
* `5`      Not a writable directory.
* `6`      Not expected argument.
* `16`     Locked runtime directory. Another nicy instance is running.
* `126`    Permission not granted without root privileges.
* `127`    Command not found.

# BUGS

Bug reports are welcome.
<https://github.com/canalguada/nicy/issues>

# SEE ALSO

`systemctl`(1), `systemd-run`(1), `sudo`(8)

`sched`(7),  `renice`(1),  `chrt`(1),  `ionice`(1), `choom(1)`

# NOTES

1.  YAML <https://en.wikipedia.org/wiki/YAML>
2.  Control Group v2
<https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html>
