# Configuration
## Files
When run by unprivileged user, nicy reads its configuration from *$XDG_CONFIG_HOME*, */etc* and */usr/local/etc* per order of precedence.

When run by superuser, only system directories are read.

When available for configuration, files in *$XDG_CONFIG_HOME* have the highest priority and files in */usr/local/etc* take precedence over files in */etc* to allow system administrator to overwrite default values.

nicy successively sources or processes in each directory:

- one nicy/**environment** file
- at least one nicy/**\*.cgroups** file
- at least one nicy/**\*.types** file
- none or many nicy/rules.d/**\*.rules** files

Except for empty lines or lines beginning with "#", which are ignored, every line of the **.cgroups**, **.types** and **.rules** [json](https://en.wikipedia.org/wiki/JSON) files contains one single json object with at least, respectively, a "cgroup", "type" or "name" key-value pair. 

When these key-value pairs appear more than once, the highest precedence mechanism is used, removing the duplicates.
```
/etc/nicy
 ├──00-cgroups.cgroups
 ├──00-types.types
 ├──environment
 └──rules.d
     └──vim.rules       [B]

$XDG_CONFIG_HOME/nicy
 ├──cgroups.cgroups
 ├──types.types
 ├──environment
 └──rules.d
     ├──temp.rules      [C]
     ├──00-default
     │   └──browsers.rules
     └──50-custom
         ├──temp.rules
         └──vim.rules   [A]

```
For instance:

- if a rule object in the file [A] share the same key-value `{"name":"nvim"}` with another rule object in the file [B], the former takes precedence over the latter.
- a rule object in the file [C] take precedence over any other rule object elsewhere with the highest precedence mechanism.

[Ananicy](https://github.com/Nefelim4ag/Ananicy) users can manually import their configuration accordingly and start using nicy.

## Cgroups
A control group (abbreviated as [cgroup](https://manpages.debian.org/testing/manpages/cgroups.7.en.html)) is a collection of processes that are bound to a set of limits or parameters defined via the cgroup filesystem.

At least one .cgroups file is required to list these available sets, one "cgroup" object per line.

For instance, `{ "cgroup": "cpu66", "CPUQuota": "66%" }` defines a **cpu66** **cgroup**, where the bound processes will share at maximum **66% of total CPU time available on ALL CORES** as per **CPUQuota** limit.

The **cgroup** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the following parameters and limits.

### CPUQuota
Assign the specified CPU time quota to the processes executed.

Takes a percentage value, suffixed with "%" or not. The percentage specifies how much CPU time the unit shall get at maximum, relative to the total CPU time available on ALL CORES.

### MemoryHigh
Specify the throttling limit on memory usage of the executed processes in this unit. **Requires the unified control group hierarchy.**

Memory usage may go above the limit if unavoidable, but the processes are heavily slowed down and memory is taken away aggressively in such cases. This is the main mechanism to control memory usage of a unit.

Takes a memory size in bytes. If the value is suffixed with K, M, G or T, the specified memory size is parsed as Kilobytes, Megabytes, Gigabytes, or Terabytes (with the base 1024), respectively.
Alternatively, a percentage value may be specified, suffixed with "%" or not, which is taken relative to the installed physical memory on the system. If assigned the special value "infinity", no memory throttling is applied.

### MemoryMax
Specify the absolute limit on memory usage of the executed processes in this unit. **Requires the unified control group hierarchy.**

If memory usage cannot be contained under the limit, out-of-memory killer is invoked inside the unit. It is recommended to use MemoryHigh= as the main control mechanism and use MemoryMax= as the last line of defense.

Same format than MemoryHigh.

### IOWeight
Set the default overall block I/O weight for the executed processes. **Requires the unified control group hierarchy.**

Takes a single weight value (between 1 and 10000) to set the default block I/O weight. This controls the "io.weight" control group attribute, which defaults to 100. For details about this control group attribute, see IO Interface Files[8]. The available I/O bandwidth is split up among all units within one slice relative to their block I/O weight.

See also [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html).

Nicy relies on [systemd slice units](https://www.freedesktop.org/software/systemd/man/systemd.slice.html) to manage cgroups at run-time.

## Types
At least one .types file is required to group sets of properties and define classes, or types, of programs.

A **type** can be seen as a **generic preset**.

For instance, whether you run a web-browser or another, you will probably set an higher scheduling priority for its processes, ad‐ just his OOM score, and limit the available CPU time.

Instead of writing twice, or more times, the same preset with almost the same properties, you can create a single **Web-Browser** type:

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "CPUQuota": "66%" }`

Or better, since many programs can run inside the same cgroup:

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "cgroup": "cpu66" }`

The **type** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the above defined for **.cgroups** files or the following keys.

### nice (see [renice(1)](https://manpages.debian.org/buster/bsdutils/renice.1.en.html))
Adjusted niceness, which affects process scheduling.

Niceness values range **from -20 to 19** (from most to least favorable to the process). **Root-credentials required** for negative niceness using default shell, and almost always with other supported shells. 

### sched (see [sched(7)](https://manpages.debian.org/buster/manpages/sched.7.en.html) and [chrt(1)](https://manpages.debian.org/buster/util-linux/chrt.1.en.html))
Available policies are:

- **other** for SCHED_NORMAL/OTHER. This is the default policy and for the average program with some interaction. Does preemption of other processes.
- **fifo** for SCHED_FIFO **(root-credentials required)**. First-In, First Out Scheduler, used only for real-time constraints. Processes in this class are usually not preempted by others, they need to free themselves from the CPU and as such you need special designed applications. **Use with extreme care.**
- **rr** for SCHED_RR **(root-credentials required)**. Round-Robin Scheduler, also used for real-time constraints. CPU-time is assigned in an round-robin fashion with a much smaller timeslice than with SCHED_NORMAL and processes in this group are favoured over SCHED_NORMAL. Usable for audio/video applications near peak rate of the system.
- **batch** for SCHED_BATCH: SCHED_BATCH was designed for non-interactive, CPU-bound applications. It uses longer timeslices (to better exploit the cache), but can be interrupted anytime by other processes in other classes to guarantee interaction of the system. Processes in this class are selected last but may result in a considerable speed-up (up to 300%). No interactive boosting is done.
- **idle** for SCHED_IDLEPRIO. SCHED_IDLEPRIO is similar to SCHED_BATCH, but was explicitly designed to consume only the time the CPU is idle. No interactive boosting is done.

### rtprio
Specify static priority required for SCHED_FIFO and SCHED_RR.

Usually ranged **from 1 to 99**.

### ioclass (see [ionice(1)](https://manpages.debian.org/buster/util-linux/ionice.1.en.html))
A process can be in one of three I/O scheduling classes:

- **idle**: a program running with idle I/O priority will only get disk time when no other program has asked for disk I/O for a defined grace period. The impact of an idle I/O process on normal system activity should be zero. This scheduling class does not take a priority argument.
- **best-effort**: this is the effective scheduling class for any process that has not asked for a specific I/O priority. This class takes a priority argument from 0-7, with a lower number being higher priority. Programs running at the same best-effort priority are served in a round-robin fashion.
- **realtime** **(root-credentials required)**: processes in the RT scheduling class is given first access to the disk, regardless of what else is going on in the system. Thus the RT class needs to be used with some care, as it can starve other processes. As with the best-effort class, 8 priority levels are defined denoting how big a time slice a given process will receive on each scheduling window.
- **none**.

### ionice
See above.

For realtime and best-effort I/O cheduling classes, 0-7 are valid data (priority levels), and 0 represents the highest priority level.

### oom_score_adjust (see [choom(1)](https://manpages.debian.org/buster/util-linux/choom.1.en.html))
Out-Of-Memory killer score setting adjustement added to the badness score, before it is used to determine which task to kill.

Acceptable values range **from -1000 to +1000**.

See also comments in the `00-types.types` file.

## Rules
Once the available cgroups and types have been set, subdirectories can be filled with some .rules files.

A **rule** can be seen as a **specific preset** for a unique program. For instance:
```
{ "name": "chromium", "type": "Web-Browser" }
{ "name": "firefox", "type": "Web-Browser" }
```
More properties can be added to rules. For instance, some other browser could quickly stand out as a memory hog when opening too many tabs. It may be useful then to control the memory available for that specific browser.
```
{ "name": "other-browser", "type": "Web-Browser", "MemoryHigh": "60%", "MemoryMax": "75%" }
```
The **name** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match neither the keys defined for **.cgroups** and **.types** files, nor the following keys.
### env
Allow to specify shell environment variables.
### cmdargs
Allow to pass arguments to the program.

## Builtin commands
### Installing scripts that replace the nicy commands
After adding a new rule and using the `rebuild` builtin command to refresh the cache, with the configuration above, `nicy run other-browser` roughly executes the script below, generated with the `show` builtin command:
```
$ nicy rebuild --force
Building json cache...
Done.

$ nicy show other-browser
#!/bin/sh
[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system=
[ $(id -u) -ne 0 ] && SUDO=sudo || SUDO=
$SUDO renice -n -3 -p $$ >/dev/null 2>&1
systemctl ${user_or_system} start nicy-cpu66.slice >/dev/null 2>&1
systemctl ${user_or_system} --runtime set-property \
       nicy-cpu66.slice CPUQuota=66% >/dev/null 2>&1
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet \
       --unit=other-browser-$$ --scope --slice=nicy-cpu66.slice \
       -p MemoryHigh=60% -p MemoryMax=75% choom -n 1000 \
       -- /usr/bin/other-browser "$@"
```

One user can then copy this script somewhere at the beginning of its PATH, turning it available as a nicy replacement for the initial other-browser command .

With `install` builtin command, after removing the duplicates and if matching an existing command, nicy generates $NICY_SHELL scripts for all the specific rules and installs them in $NICY_SCRIPTS.

### Managing the running processes of the current user
 nicy can scan the running processes looking for properties to apply in order to match existing specific rules.

For instance, after launching `pcmanfm-qt` file manager and using the `list` builtin command to check existing rule and type, nicy can adjust the niceness and I/O scheduling class properties, then move any process in the pcmanfm-qt process group to its proper scope, with `manage` builtin command.

```
$ /usr/bin/pcmanfm-qt &>/dev/null &
[1] 2686

$ cat /proc/2686/cgroup
0::/user.slice/user-1001.slice/session-2.scope

$ nicy list rules | grep pcmanfm-qt
pcmanfm-qt     {"name":"pcmanfm-qt","type":"File-Manager"}

$ nicy list types | grep File-Manager
File-Manager   {"type":"File-Manager","nice":-3,"ioclass":"best-effort","cgroup":"cpu33"}

$ nicy manage --dry-run --user
nicy: manage: adjusting comm:pcmanfm-qt pgrp:2686 cgroup:session-3.scope pids:2686
nicy: manage: sudo renice -n -3 -g 2686
nicy: manage: ionice -c 2 -P 2686
nicy: manage: systemctl --user start nicy-cpu33.slice
nicy: manage: systemctl --user --runtime set-property nicy-cpu33.slice CPUQuota=33%
nicy: manage: busctl call --quiet --user org.freedesktop.systemd1 \
    /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager \
    StartTransientUnit ssa(sv)a(sa(sv)) pcmanfm-qt-2686.scope \
    fail 2 PIDs au 1 2686 Slice s nicy-cpu33.slice 0

$ systemd-cgls /user.slice/user-1001.slice/user@1001.service/nicy.slice
Control group /user.slice/user-1001.slice/user@1001.service/nicy.slice:
└─nicy-cpu33.slice
 └─pcmanfm-qt-2686.scope
   └─2686 /usr/bin/pcmanfm-qt
```

nicy can adjust the properties of almost any process, whether it runs in user or in system slice.

### Managing the processes in system slice as superuser
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

### Managing the processes of all users as superuser
```
# nicy manage --dry-run --global
nicy: manage: adjusting comm:lightdm pgrp:541 cgroup:session-3.scope pids:1310
nicy: manage: ionice -c 1 -n 4 -P 541
nicy: manage: adjusting comm:pulseaudio pgrp:1467 cgroup:pulseaudio.service pids:1467
nicy: manage: ionice -c 1 -P 1467
nicy: manage: chrt --rr -a -p 1 1467
nicy: manage: adjusting comm:i3 pgrp:1485 cgroup:session-3.scope pids:1485
nicy: manage: renice -n -10 -g 1485
nicy: manage: ionice -c 1 -P 1485
nicy: manage: adjusting comm:mopidy pgrp:3032 cgroup:mopidy.service pids:3032
nicy: manage: ionice -c 1 -P 3032
nicy: manage: adjusting comm:pcmanfm-qt pgrp:9063 cgroup:session-3.scope pids:9064
nicy: manage: renice -n -3 -g 9063
nicy: manage: runuser -u canalguada -- env \
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus \
    XDG_RUNTIME_DIR=/run/user/1001 \
    systemctl --user start nicy-cpu33.slice
nicy: manage: runuser -u canalguada -- env \
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus \
    XDG_RUNTIME_DIR=/run/user/1001 \
    systemctl --user --runtime set-property nicy-cpu33.slice CPUQuota=33%
nicy: manage: runuser -u canalguada -- env \
    DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/1001/bus \
    XDG_RUNTIME_DIR=/run/user/1001 \
    busctl call --quiet --user \
    org.freedesktop.systemd1 /org/freedesktop/systemd1 \
    org.freedesktop.systemd1.Manager StartTransientUnit 'ssa(sv)a(sa(sv))' \
    pcmanfm-qt-9064.scope fail 2 PIDs au 1 9064 Slice s nicy-cpu33.slice 0
```
See also comments in the `environment` file.

## Environment variables
nicy checks for the existence of these variables in each **environment** file.

### XDG_CONFIG_HOME
Base directory relative to which user specific configuration files should be stored.
If either not set or empty, defaults to `$HOME/.config`.

### JQ_PATH
Path to the jq executable.
Defaults to `$(command -v jq)`.

### NICY_CONF
Paths to the configuration directories.
Defaults to `($HOME/.config/nicy /usr/local/etc/nicy /etc/nicy)`, or `(/usr/local/etc/nicy /etc/nicy)` when the program is run by superuser.

### NICY_DATA
Path to the read-only data not edited by user.
Defaults to `%prefix%/share/nicy`.

### NICY_LIB
Path to the program library.
Defaults to `%prefix%/lib/nicy`.

### NICY_VERBOSE
Be verbose and show every launched command.
Defaults to `yes`.

### NICY_SHELL
Shell used when generating the scripts.
Defaults to `/bin/bash`.

### NICY_SUDO
Sudo command used when the root-credentials are required.
Defaults to `sudo`.

### NICY_SCRIPTS
Path to directory where the scripts are installed.
Defaults to `$HOME/bin/nicy` or `/usr/local/bin/nicy` for superuser.

### NICY_IGNORE
Path to the file that lists the commands to ignore when installing the scripts.
Defaults to `${NICY_CONF[0]}/ignore`.

### NICY_SYMLINK
Path to the file that lists the commands to symlink after installing the scripts.
Defaults to `${NICY_CONF[0]}/symlink`.
