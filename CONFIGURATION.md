# Configuration

`nicy` supports several mechanisms for supplying/obtaining configuration and
run-time parameters: command line options, environment variables, the
config.yaml configuration files and fallback defaults. When the same information
is supplied in more than one way, the highest precedence mechanism is used. The
list of mechanisms is ordered from highest precedence to lowest.

Note that not all parameters can be supplied via all methods. The available command line options and environment variables (and some defaults) are described in the
`nicy`(1) page. Most configuration file parameters, with their defaults, are
described below.

## Files

`nicy` uses a configuration file called *config.yaml* in YAML format. The
*config.yaml* configuration file is searched for in the following places when the program
is started as a normal user:

* $XDG_CONFIG_HOME/nicy
* /usr/local/etc/nicy
* /etc/nicy

When `nicy` is started by the “root” user, the config file search locations
are as follows:

* /usr/local/etc/nicy
* /etc/nicy

When more than one *config.yaml* are found, `nicy` merges their content per
order of precedence.

## Cgroups object

A control group, abbreviated as [cgroup(7)](https://manpages.debian.org/testing/manpages/cgroups.7.en.html), is a collection of processes that
are bound to a set of limits or parameters.

For instance:

```yaml
cgroups:
  cpu66:
    CPUQuota: 66%
  cpu80:
    CPUQuota: 80%
```

defines a `cpu66` cgroup and a `cpu80` where the respectively bound processes
will share at maximum 66% and 80% of total CPU time available on ALL CORES as
per their `CPUQuota` limit.

All key-value pairs are optional and will be ignored if the key doesn't match the
following parameters and limits.

### CPUQuota:

Assign the specified CPU time quota to the processes executed.

Takes a percentage value, suffixed with "%" or not. The percentage
specifies how much CPU time the unit shall get at maximum, relative to
the total CPU time available on ALL CORES.

### MemoryHigh:

Specify the throttling limit on memory usage of the executed processes
in this unit. `Requires the unified control group hierarchy`.

Memory usage may go above the limit if unavoidable, but the
processes are heavily slowed down and memory is taken away aggressively in
such cases. This is the main mechanism to control memory usage of a unit.

Takes a memory size in bytes. If the value is suffixed with K, M, G or T, the
specified memory size is parsed as Kilobytes, Megabytes, Gigabytes, or
Terabytes (with the base 1024), respectively. Alternatively, a percentage
value may be specified, suffixed with "%" or not, which is taken relative to
the installed physical memory on the system. If assigned the special value
"infinity", no memory throttling is applied.

### MemoryMax:

Specify the absolute limit on memory usage of the executed processes in this
unit. `Requires the unified control group hierarchy`.

If memory usage cannot be contained under the limit, out-of-memory killer is
invoked inside the unit. It is recommended to use `MemoryHigh=` as the main
control mechanism and use `MemoryMax=` as the last line of defense.

Same format than MemoryHigh.

### IOWeight:

Set the default overall block I/O weight for the executed processes.
`Requires the unified control group hierarchy`.

Takes a single weight value (between 1 and 10000) to set the default block I/O
weight. This controls the "io.weight" control group attribute, which defaults
to 100. For details about this control group attribute, see
[IO Interface Files](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html#io-interface-files).
The available I/O bandwidth is split up among all units within one slice
relative to their block I/O weight.

See also [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html). `nicy` relies on [systemd slice units](https://www.freedesktop.org/software/systemd/man/systemd.slice.html) to manage cgroups at run-time.

## Appgroups object

A `profile` can be seen as a `generic preset`.

For instance, whether you run a web-browser or another, you will probably set
an higher scheduling priority for its processes, adjust his OOM score, and
limit the available CPU time. Instead of writing twice, or more times, the
same preset with almost the same properties, you can create a single
`Web-Browser profile`:

```yaml
appgroups:
  Web-Browser:
    profile:
      nice: -3
      oom_score_adjust: 1000
      cgroup: cpu66
```

All key-value pairs are optional and will be ignored if the key doesn't match the
requirements above defined for *cgroups* or below for the following properties.

### nice:

Adjusted niceness, which affects process scheduling.

Niceness values range `from -20 to 19` (from most to least favorable to the
process). `Root-credentials required` for negative niceness value with default
shell, and almost always with other supported shells.
See [renice(1)](https://manpages.debian.org/testing/bsdutils/renice.1.en.html).

### sched:

Available policies are:

* `other` for `SCHED_NORMAL/OTHER`.
This is the default policy and for the average program with some interaction.
Does preemption of other processes.
* `fifo` for `SCHED_FIFO`. `Root-credentials required`.
First-In, First Out Scheduler, used only for real-time constraints. Processes
in this class are usually not preempted by others, they need to free
themselves from the CPU and as such you need special designed applications.
`Use with extreme care`.
* `rr` for `SCHED_RR`. `Root-credentials required`.
Round-Robin Scheduler, also used for real-time constraints. CPU-time is
assigned in an round-robin fashion  with a much smaller timeslice than with
SCHED_NORMAL and processes in this group are favoured over SCHED_NORMAL.
Usable for audio/video applications near peak rate of the system.
.* `batch` for `SCHED_BATCH`.
SCHED_BATCH was designed for non-interactive, CPU-bound applications. It uses
longer timeslices (to better exploit the cache), but can be interrupted
anytime by other processes in other classes to guarantee interaction of the
system. Processes in this class are selected last but may result in a
considerable speed-up (up to 300%). No interactive boosting is done.
* `idle` for `SCHED_IDLEPRIO`.
SCHED_IDLEPRIO is similar to SCHED_BATCH, but was explicitly designed to
consume only the time the CPU is idle. No interactive boosting is done.

### rtprio:

Specify static priority required for `SCHED_FIFO` and `SCHED_RR`.
Usually range `from 1 to 99`.

See [sched(7)](https://manpages.debian.org/testing/manpages/sched.7.en.html)
and [chrt(1)](https://manpages.debian.org/testing/util-linux/chrt.1.en.html).

### ioclass:

A process can be in one of three I/O scheduling classes:

* `idle`:
a program running with idle I/O priority will only get disk time when no other
program has asked for disk I/O for a defined grace period. The impact of an
idle I/O process on normal system activity should be zero. This scheduling
class does not take a priority argument.
* `best-effort`:
this is the effective scheduling class for any process that has not asked for
a specific I/O priority. This class takes a priority argument from `0 to 7`,
with a lower number being higher priority. Programs running at the same
best-effort priority are served in a round-robin fashion.
* `realtime (root-credentials required)`:
processes in the RT scheduling class is given first access to the disk,
regardless of what else is going on in the system. Thus the RT class needs to
be used with some care, as it can starve other processes. As with the
best-effort class, 8 priority levels are defined denoting how big a time slice
a given process will receive on each scheduling window.
* `none`.

### ionice:

For realtime and best-effort I/O cheduling classes, `0-7` are valid data
(priority levels), and 0 represents the highest priority level.

See [ionice(1)](https://manpages.debian.org/testing/util-linux/ionice.1.en.html).

### oom_score_adjust:

Out-Of-Memory killer score setting adjustement added to the badness score,
before it is used to determine which task to kill.

Acceptable values range `from -1000 to +1000`.

See [choom(1)](https://manpages.debian.org/testing/util-linux/choom.1.en.html).

## Rules object

Once the available `cgroups` and `profiles` have been set, we can assigned
`rules` to programs.

A `rule` can be seen as a `specific preset` for a unique program.

For instance:

```yaml
appgroups:
  Web-Browser:
    profile:
      nice: -3
      oom_score_adjust: 1000
      cgroup: cpu66
      type: Web-Browser
    assignments:
      - chromium
      - firefox
      - other-browser
  Doc-View:
    profile:
      cgroup: cpu80
      nice: -3
    assignments:
      - nvim
  none:
    assignments:
      - nvim-qt
rules:
  other-browser:
    Memory-High: 60%
    Memory-Max: 75%
  nvim:
    cmdargs: ["--listen", "/tmp/nvimsocket"]
    env: {SHELL: "/bin/bash"}
  nvim-qt:
    cgroup: cpu66
    cmdargs: ["--nofork", "--nvim=/usr/bin/nvim"]
    env: {SHELL: "/bin/bash"}
```

First, programs are assigned to a type, then more properties can be added.

All key-value pairs are optional and will be ignored if the key doesn't match
the keys available for cgroups and types, plus cgroup and profile, and the following
keys.

### env:

Allow to specify shell environment variables.

### cmdargs:

Allow to pass extra arguments to the program.

## Builtin commands

### Installing scripts that replace the nicy commands

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

See Environment variables below.

### Managing the running processes of the current user

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
File-Manager    user    {"nice":-3,"ioclass":"best-effort","cgroup":"cpu80"}

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

### Setting the processes in system slice as superuser

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

### Setting the processes of all users as superuser

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

## Environment variables

`nicy` checks for the existence of these variables in environment.

### XDG_CONFIG_HOME
Base directory relative to which user specific configuration files should be
stored. If either not set or empty, defaults to *$HOME/.config*.

### NICY_VERBOSE
Be verbose and show every launched command. Defaults to *false*.

### NICY_SHELL
Shell used when generating the scripts. Defaults to */bin/bash*.

### NICY_SUDO
Command used when the root-credentials are  required. Defaults to `sudo`(8).

### NICY_SCRIPTS_LOCATION
Path to directory where the scripts are installed. Defaults to
*$HOME/bin/nicy* or */usr/local/bin/nicy* for superuser.

