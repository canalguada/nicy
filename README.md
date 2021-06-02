# Nicy
## About
Set the execution environment and configure the resources that spawned and running processes are allowed to share.
## Why
My legacy low-end hardware quickly turns nasty, running hot then shutting down, when launching too many or modern "resource hungry" softwares. But controlling the resources that some of them request help that hardware to be serviceable again.

I used to install [Ananicy](https://github.com/Nefelim4ag/Ananicy), an auto nice daemon, with community rules support, that relies too on the Linux Control Groups ([Cgroups](https://en.wikipedia.org/wiki/Cgroups)).

I write nicy because I need to control the available resources per program according to some more context, in other words adding options in a command line, not editing a configuration file as a privileged user.

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
    Show the effective script for these COMMAND and OPTIONS, if any.
  nicy list [OPTION] CATEGORY
    List the objects from this CATEGORY, removing all duplicates.
  nicy install [OPTION]...
    Install scripts.
  nicy rebuild [OPTION]
    Rebuild the json cache and exit.
  nicy manage [OPTION]...
    Apply available presets, managing the running processes.

Common options:
  -h, --help           display this help and exit
  -v, --version        show the program version and exit

Run and show options:
  -q, --quiet          suppress additional output
  -v, --verbose        display which command is launched
  -p, --preset=PRESET  apply this PRESET
  -d, --default        like '--preset=default'
  -z, --cgroup-only    like '--preset=cgroup-only'
  -c, --cgroup=CGROUP  run inside this CGROUP
      --cpu<QUOTA>     like '--cgroup=cpu<QUOTA>'
  -m, --managed        always run inside a scope
  -u, --force-cgroup   run inside a matching cgroup, if any

Run only options:
  -n, --dry-run        display commands but do not run them

List options:
  -f, --from=CONFDIR   list only objects from CONFDIR directory

Install options:
      --shell=SHELL    generate script for SHELL
      --path=DESTDIR   install inside DESTDIR

Rebuild options:
      --force          ignore existing files in cache

Manage options:
      --user           inside the slice of the current user
      --system         inside the system slice
      --global         inside the global user slice
      --all            inside either system or global user slice
  -n, --dry-run        display commands but do not run them

The PRESET argument can be: 'auto' to apply the specific preset for
the command, its rule, if any; 'cgroup-only' to apply only the cgroup
properties of that rule, if found; 'default' to apply the special
fallback preset; or any generic preset, a type.

The CGROUP argument can be a cgroup defined in configuration files.

The QUOTA argument can be an integer ranging from 1 to 99 that represents
a percentage relative to the total CPU time available on all cores.

The CATEGORY argument can be 'rules', 'types' or 'cgroups', matching
the extensions of configuration files.

The CONFDIR argument can be one out of NICY_SEARCH_DIRS directories. When
filtering per CONFDIR, no duplicate is removed taking into account the
priority between directories.

The SHELL argument can be a path to a supported shell (sh, dash, bash,
zsh). Default value is /bin/sh.

The scripts are installed, when a specific rule is available, in DESTDIR,
if given. Default value is $HOME/bin/nicy, or /usr/local/bin/nicy for
system (root) user.

The processes are managed when a specific rule is available.

The --system, --global and --all options require root credentials.

```

## Requirements
* [systemd](https://systemd.io/) and [systemd-run](https://www.freedesktop.org/software/systemd/man/systemd-run.html)
* [jq](https://stedolan.github.io/jq/), a lightweight and flexible command-line JSON processor

Most of cgroup settings are supported only with the unified control group hierarchy, the new version of kernel control group interface, see [Control Groups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html).
## Installation
Install in /usr/local: `$ sudo make install`
Install in /usr: `$ sudo make prefix=/usr install`

## Configuration
With default configuration, nicy looks for files in the following directories, in this order of preference :

- $XDG_CONFIG_HOME/nicy
- /etc/nicy
- /usr/local/etc/nicy

4 kinds of files are successively sourced or parsed :

- one **environment** file, at each directory root, to adjust the default values for some environment variables
- at least one **.cgroups** file and one **.types** file, at each directory root
- none or many **.rules** files in any subdirectory

When parsing a **.cgroups**, **.types** or **.rules** [json](https://en.wikipedia.org/wiki/JSON) configuration file, its content gets the same priority than its pathname when sorted by descending order.

Many json objects can share the same id (a **cgroup**, a **type** or a **name**) since, after reading the first, the others duplicates will be ignored.
```
/usr/local/etc/nicy
├──00-cgroups.cgroups
├──00-types.types
└──environment

/home/user/.config/nicy
├──00-cgroups.cgroups
├──00-types.types
├──environment
└──rules.d
    ├──temp.rules
    ├──00-default
    │   ├──browsers.rules
    │   └──debian.rules
    └──50-custom
        ├──aria2c.rules
        ├──mopidy.rules
        ├──ncmpcpp.rules
        ├──temp.rules
        ├──tmux.rules
        └──vim.rules
```
For instance, any rule with a given name id in `temp.rules` have an higher priority than any other rule with the same id from a file in `50-custom` directory, and any type defined in `/home/user/.config/nicy` take precedence on any type sharing the same id from `/usr/local/etc/nicy`.

[Ananicy](https://github.com/Nefelim4ag/Ananicy) users can manually import their configuration accordingly and start using nicy.

### Cgroups
>   A control group (abbreviated as [cgroup](https://en.wikipedia.org/wiki/Cgroups)) is a collection of processes that are bound by the same criteria and associated with a set of parameters or limits.

The .cgroups file lists in json format such sets of parameters, or limits, one for each cgroup. For instance :

`{ "cgroup": "cpu66", "CPUQuota": "66%" }`

 defines a **cpu66** **cgroup**, where the bound processes will share at maximum **66% of total CPU time available on ALL CORES** as per **CPUQuota** limit.

The **cgroup** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the following parameters and limits :

- **CPUQuota**. Assign the specified CPU time quota to the processes executed. Takes a percentage value, suffixed with "%" or not. The percentage specifies how much CPU time the unit shall get at maximum, relative to the total CPU time available on ALL CORES.
- **MemoryHigh**. Specify the throttling limit on memory usage of the executed processes in this unit. Memory usage may go above the limit if unavoidable, but the processes are heavily slowed down and memory is taken away aggressively in such cases. This is the main mechanism to control memory usage of a unit. Takes a memory size in bytes. If the value is suffixed with K, M, G or T, the specified memory size is parsed as Kilobytes, Megabytes, Gigabytes, or Terabytes (with the base 1024), respectively. Alternatively, a percentage value may be specified, suffixed with "%" or not, which is taken relative to the installed physical memory on the system. If assigned the special value "infinity", no memory throttling is applied. **Requires the unified control group hierarchy.**
- **MemoryMax**. Specify the absolute limit on memory usage of the executed processes in this unit. If memory usage cannot be contained under the limit, out-of-memory killer is invoked inside the unit. It is recommended to use MemoryHigh= as the main control mechanism and use MemoryMax= as the last line of defense. Same format than MemoryHigh. **Requires the unified control group hierarchy.**
- **IOWeight**. Set the default overall block I/O weight for the executed processes. Takes a single weight value (between 1 and 10000) to set the default block I/O weight. This controls the "io.weight" control group attribute, which defaults to 100. For details about this control group attribute, see IO Interface Files[8]. The available I/O bandwidth is split up among all units within one slice relative to their block I/O weight. **Requires the unified control group hierarchy.**

See also [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html).

Nicy relies on [systemd slice units](https://www.freedesktop.org/software/systemd/man/systemd.slice.html) to manage cgroups at run-time : a .cgroups file is required to declare the available cgroups.
### Types
Writing a full rule for each program would turn boring.

The .types file groups sets of properties in order to define classes of programs, or types. A **type** can be seen as a generic **preset**. For instance, whether you run `nicy chromium` or `nicy firefox` you will probably set an higher scheduling priority for all processes, adjust the OOM score, and limit the available CPU time. Instead of writing twice, or more times, the same rule repeating almost the same properties, one create a single type **Web-Browser** like this one.

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "CPUQuota": "66%" }`

Or better, since many programs can run inside the same cgroup, like below :

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "cgroup": "cpu66" }`

The **type** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the previously defined or the following keys :

- **nice** ([renice(1)](https://manpages.debian.org/buster/bsdutils/renice.1.en.html)). Adjusted niceness, which affects process scheduling. Niceness values range from -20 (most favorable to the process) to 19 (least favorable to the process) (**root-credentials required** for negative niceness with NICY_SHELL default value).
- **sched** ([sched(7)](https://manpages.debian.org/buster/manpages/sched.7.en.html) and [chrt(1)](https://manpages.debian.org/buster/util-linux/chrt.1.en.html)). Available policies are :
	- ***other*** for SCHED_NORMAL/OTHER. This is the default policy and for the average program with some interaction. Does preemption of other processes.
	- ***fifo*** for SCHED_FIFO **(root-credentials required)**. First-In, First Out Scheduler, used only for real-time constraints. Processes in this class are usually not preempted by others, they need to free themselves from the CPU and as such you need special designed applications. **Use with extreme  care.**
	- ***rr*** for SCHED_RR **(root-credentials required)**. Round-Robin Scheduler, also used for real-time constraints. CPU-time is assigned in an round-robin fashion  with a much smaller timeslice than with SCHED_NORMAL and processes in this group are favoured over SCHED_NORMAL. Usable for audio/video applications near peak rate of the system.
	- ***batch*** for SCHED_BATCH: SCHED_BATCH was designed for non-interactive, CPU-bound applications. It uses longer timeslices (to better exploit the cache), but can be interrupted anytime by other processes in other classes to guarantee interaction of the system. Processes in this class are selected last but may result in a considerable speed-up (up to 300%). No interactive boosting is done.
	- ***idle*** for SCHED_IDLEPRIO. SCHED_IDLEPRIO is similar to SCHED_BATCH, but was explicitly designed to consume only the time the CPU is idle. No interactive boosting is done.
- **rtprio**. Specify static priority required for SCHED_FIFO and SCHED_RR. Usually ranged from 1-99.
- **ioclass** ([ionice(1)](https://manpages.debian.org/buster/schedtool/schedtool.8.en.html)). A process can be in one of three I/O scheduling classes:
	- ***idle***: a program running with idle I/O priority will only get disk time when no other program has asked for disk I/O for a defined grace period. The impact of an idle I/O process on normal system activity should be zero. This scheduling class does not take a priority argument.
	- ***best-effort***: this is the effective scheduling class for any process that has not asked for a specific I/O priority. This class takes a priority argument from 0-7, with a lower number being higher priority. Programs running at the same best-effort priority are served in a round-robin fashion.
	- ***realtime*** **(root-credentials required)**: processes in the RT scheduling class is given first access to the disk, regardless of what else is going on in the system. Thus the RT class needs to be used with some care, as it can starve other processes. As with the best-effort class, 8 priority levels are defined denoting how big a time slice a given process will receive on each scheduling window.
	- ***none***.
- **ionice** (see above). For realtime and best-effort I/O cheduling classes, 0-7 are valid data (priority levels), and 0 represents the highest priority level.
- **oom_score_adjust** ([choom(1)](https://manpages.debian.org/buster/schedtool/schedtool.8.en.html)). Out-Of-Memory killer score setting adjustement added to the badness score, before it is used to determine which task to kill. Acceptable values range from -1000 to +1000.

See also comments in the `00-types.types` file.
### Rules
Once the available cgroups and types have been set, subdirectories can be filled with some .rules files. A **rule** can be seen as a specific **preset** for a unique program. For instance :
```
{ "name": "chromium", "type": "Web-Browser" }
{ "name": "firefox", "type": "Web-Browser" }
```
More properties can be added to rules. For instance, some random browser could quickly stand out as a memory hog when opening too many tabs. It may be useful then to control the memory available for that specific browser.
```
{ "name": "cheap-browser", "type": "Web-Browser", "MemoryHigh": "60%", "MemoryMax": "75%" }
```
The **name** pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match neither the keys defined for .cgroups and .types files,, nor:
- **env**. Allow to specify shell environment variables.
- **cmdargs**. Allow to pass arguments to the program.

## Subcommands
After adding a new rule and using the **rebuild** subcommand to refresh the cache, with the configuration above, **nicy run cheap-browser** roughly executes the script below, generated with the **show** subcommand :
```
$ nicy show cheap-browser
#!/bin/sh
[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system=
[ $(id -u) -ne 0 ] && SUDO=sudo || SUDO=
$SUDO renice -n -3 -p $$ >/dev/null 2>&1
systemctl ${user_or_system} start nicy-cpu66.slice >/dev/null 2>&1
systemctl ${user_or_system} --runtime set-property nicy-cpu66.slice CPUQuota=66% >/dev/null 2>&1
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet --unit=cheap-browser-$$ --scope --slice=nicy-cpu66.slice -p MemoryHigh=60% -p MemoryMax=75% choom -n 1000 -- /usr/bin/cheap-browser "$@"
```
With `$HOME/bin/nicy` added at the beginning of his PATH environment variable, one user can copy there this script, making it available as a replacement for the initial `cheap-browser` command .

With **install** subcommand, after removing the duplicates and if matching an existing command, scripts for all the specific rules are generated and installed in configured directory.
```
$ ls $HOME/bin/nicy
cheap-browser.nicy
chromium.nicy
firefox.nicy
```

With **manage** subcommand, the running processes are scanned looking for properties to apply, in order to match existing specific rules. For instance, after launching `pcmanfm-qt` file manager and using the **list** subcommand to check existing rule and type :
```
$ /usr/bin/pcmanfm-qt &>/dev/null &
[1] 2686
$ nicy list rules |grep pcmanfm-qt
pcmanfm-qt     {"name":"pcmanfm-qt","type":"File-Manager"}
$ nicy list types |grep File-Manager
File-Manager   {"type":"File-Manager","nice":-3,"ioclass":"best-effort","cgroup":"cpu33"}
$ nicy manage --dry-run --user
nicy: manage: adjusting comm:pcmanfm-qt pgrp:2686 cgroup:session-3.scope pids:2686
nicy: manage: sudo renice -n -3 -g 2686
nicy: manage: ionice -c 2 -P 2686
nicy: manage: systemctl --user start nicy-cpu33.slice
nicy: manage: systemctl --user --runtime set-property nicy-cpu33.slice CPUQuota=33%
nicy: manage: busctl call --quiet --user org.freedesktop.systemd1 /org/freedesktop/systemd1 org.freedesktop.systemd1.Manager StartTransientUnit ssa(sv)a(sa(sv)) pcmanfm-qt-2686.scope fail 2 PIDs au 1 2686 Slice s nicy-cpu33.slice 0
$ systemd-cgls /user.slice/user-1001.slice/user@1001.service/nicy.slice
Control group /user.slice/user-1001.slice/user@1001.service/nicy.slice:
└─nicy-cpu33.slice 
  └─pcmanfm-qt-2686.scope 
    └─2686 /usr/bin/pcmanfm-qt
```

See also comments in the `environment` file.

## TODO
- Write a man page.
- Build a debian package.
- Switch to python (the easy way) before reaching 2k LOC.

