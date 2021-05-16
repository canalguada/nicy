# Nicy
## About
A bash script that configures the execution environment and the resources that the spawned processes of the command are allowed to share.
## Why
My legacy low-end hardware quickly turns nasty, running hot then shutting down, when launching too many or modern "resource hungry" softwares. But controlling the resources that some of them request help that hardware to be servieable again.

I used to install [Ananicy](https://github.com/Nefelim4ag/Ananicy), an auto nice daemon, with community rules support, that relies too on the Linux Control Groups ([Cgroups](https://en.wikipedia.org/wiki/Cgroups)).

I write nicy because I need to control the available resources per program according to some more context, in other words adding options in a command line, not editing a configuration file as a privileged user.

## Usage
```
$ nicy run --dry-run nvim-qt
nicy: sudo renice -n -3 -p 19696
nicy: systemctl --user start nicy-cpu66.slice
nicy: systemctl --user --runtime set-property nicy-cpu66.slice CPUQuota=66%
nicy: exec systemd-run --user -G -d --no-ask-password --quiet --unit=nvim-qt-19696 --scope --slice=nicy-cpu66.slice -E 'SHELL=/bin/bash -l' /usr/bin/nvim-qt --nofork --nvim /usr/bin/nvim

$ nicy show nvim-qt
#!/bin/sh
[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system=
[ $(id -u) -ne 0 ] && SUDO=sudo || SUDO=
$SUDO renice -n -3 -p $$ >/dev/null 2>&1
systemctl ${user_or_system} start nicy-cpu66.slice >/dev/null 2>&1
systemctl ${user_or_system} --runtime set-property nicy-cpu66.slice CPUQuota=66% >/dev/null 2>&1
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet --unit=nvim-qt-$$ --scope --slice=nicy-cpu66.slice -E 'SHELL=/bin/bash -l' /usr/bin/nvim-qt --nofork --nvim /usr/bin/nvim "$@"

$ nicy --help
nicy version 0.1.0

Set the execution environment and the available resources.

Usage:
  nicy run [OPTIONS]... COMMAND [ARGUMENTS]...
    Run the COMMAND in a pre-set execution environment.
  nicy show [OPTIONS]... COMMAND
    Show the effective script for the given COMMAND and OPTIONS.
  nicy list [OPTIONS] CATEGORY
    List the objects from this CATEGORY, removing all duplicates.
  nicy install [OPTIONS] [SHELL]
    Install scripts, using SHELL if any.
  nicy rebuild
    Rebuild the json cache and exit.

Common options:
  -h, --help           Display this help and exit

Run and show options:
  -q, --quiet          Suppress additional output
  -v, --verbose        Display which command is launched
  -p, --preset=PRESET  Apply this PRESET
  -d, --default        Like '--preset=default'
  -z, --cgroup-only    Like '--preset=cgroup-only'
  -c, --cgroup=CGROUP  Run inside this CGROUP
      --cpu<QUOTA>     Like '--cgroup=cpu<QUOTA>'
  -m, --managed        Run inside a scope, required or not
  -u, --force-cgroup   Run inside a matching cgroup, if any

Run only options:
  -n, --dry-run        Display commands but do not run them

List options:
  -f, --from=DIRECTORY List only objects from this DIRECTORY

The PRESET argument can be: 'auto' to apply the specific preset for
the command, its rule, if any; 'cgroup-only' to apply only the cgroup
properties of that rule, if found; 'default' to apply the special
fallback preset; or any generic preset, a type.

The CGROUP argument can be a cgroup defined in configuration files.

The QUOTA argument can be an integer ranging from 1 to 99 that represents
a percentage relative to the total CPU time available on all cores.

The CATEGORY argument can be 'rules', 'types' or 'cgroups', matching
the extensions of configuration files.

The DIRECTORY argument can be a path from NICY_SEARCH_DIRS setting. When
filtering per DIRECTORY, no duplicate is removed taking into account
the priority between directories.

The SHELL argument can be a path to a supported shell (sh, dash, bash,
zsh). Default value is /bin/sh.

Only installs scripts for rules matching a command found.

```

## Requirements
* [systemd](https://systemd.io/) and [systemd-run](https://www.freedesktop.org/software/systemd/man/systemd-run.html)
* [jq](https://stedolan.github.io/jq/), a lightweight and flexible command-line JSON processor
* [schedtool](https://github.com/freequaos/schedtool), a tool to change or query all CPU-scheduling policies

Most of cgroup settings are supported only with the unified control group hierarchy, the new version of kernel control group interface, see [Control Groups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html). 
## Installation
Install in /usr/local.
`$ sudo make install`

Install in /usr.
`$ sudo make prefix=/usr install`
## Configuration
With default configuration, nicy looks for files in the following directories, in this order of preference :

- $XDG_CONFIG_HOME/nicy
- /etc/nicy
- /usr/local/etc/nicy

4 kinds of files are successively sourced or parsed :

- one **environment** file, at each directory root, to adjust the default values for some environment variables
- at least one **.cgroups** file and one **.types** file, at each directory root
- none or many **.rules** files in any subdirectory

When parsing a **.cgroups**, **.types** or **.rules** [json](https://en.wikipedia.org/wiki/JSON) configuration file, its content gets the same priority than its pathnames when sorted by descending order.

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

 defines a **cpu66** cgroup, where the bound processes will share at maximum **66% of total CPU time available on ALL CORES** as per **CPUQuota** limit.

The *cgroup* pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the following parameters and limits :

- **CPUQuota**. Assign the specified CPU time quota to the processes executed. Takes a percentage value, suffixed with "%" or not. The percentage specifies how much CPU time the unit shall get at maximum, relative to the total CPU time available on ALL CORES.
- **MemoryHigh**. Specify the throttling limit on memory usage of the executed processes in this unit. Memory usage may go above the limit if unavoidable, but the processes are heavily slowed down and memory is taken away aggressively in such cases. This is the main mechanism to control memory usage of a unit. Takes a memory size in bytes. If the value is suffixed with K, M, G or T, the specified memory size is parsed as Kilobytes, Megabytes, Gigabytes, or Terabytes (with the base 1024), respectively. Alternatively, a percentage value may be specified, suffixed with "%" or not, which is taken relative to the installed physical memory on the system. If assigned the special value "infinity", no memory throttling is applied. **Requires the unified control group hierarchy.**
- **MemoryMax**. Specify the absolute limit on memory usage of the executed processes in this unit. If memory usage cannot be contained under the limit, out-of-memory killer is invoked inside the unit. It is recommended to use MemoryHigh= as the main control mechanism and use MemoryMax= as the last line of defense. Same format than MemoryHigh. **Requires the unified control group hierarchy.**
- **IOWeight**. Set the default overall block I/O weight for the executed processes. Takes a single weight value (between 1 and 10000) to set the default block I/O weight. This controls the "io.weight" control group attribute, which defaults to 100. For details about this control group attribute, see IO Interface Files[8]. The available I/O bandwidth is split up among all units within one slice relative to their block I/O weight. **Requires the unified control group hierarchy.**

See also [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html).

Nicy relies on [systemd slice units](https://www.freedesktop.org/software/systemd/man/systemd.slice.html) to manage cgroups at run-time : a .cgroups file is required to declare the available cgroups.
### Types
Writing a full rule for each program would turn boring.

The .types file groups sets of properties in order to define classes of programs, or types. A **type** can be seen as a generic **preset**. For instance, whether you run `nicy chromium` or `nicy firefox` you will probably set an higher scheduling priority for all processes, adjust the OOM score, and limit the available CPU time. Instead of writing twice, or more times, the same rule repeating almost the same properties, one create a single **type** **Web-Browser** like this one.

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "CPUQuota": "66%" }`

Or better, since many programs can run inside the same cgroup, like below :

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "cgroup": "cpu66" }`

The *type* pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match the previously defined or the following keys :

- **nice** ([renice(1)](https://manpages.debian.org/buster/bsdutils/renice.1.en.html)). Adjusted niceness, which affects process scheduling. Niceness values range from -20 (most favorable to the process) to 19 (least favorable to the process) (**root-credentials required** for negative niceness with NICY_SHELL default value).
- **sched** ([sched(7)](https://manpages.debian.org/buster/manpages/sched.7.en.html) and [schedtool(8)](https://manpages.debian.org/buster/schedtool/schedtool.8.en.html)). Available policies are :
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
Once the available cgroups and types have been set, subdirectories can be filled with some .rules files. A **rule** can be seen as a specific **preset** for a unique given program. For instance :
```
{ "name": "chromium", "type": "Web-Browser" }
{ "name": "firefox", "type": "Web-Browser" }
```
More properties can be added to rules. For instance, some random browser could quickly stand out as a memory hog when opening too many tabs. It may be useful then to control the memory available for that specific browser.
```
{ "name": "cheap-browser", "type": "Web-Browser", "MemoryHigh": "60%", "MemoryMax": "75%" }
```
The *name* pair is mandatory. All other key-value pairs are optional and will be ignored if the key doesn't match neither the keys defined for .cgroups and .types files, nor *env* and *cmdargs* keys, whose pairs allow respectively to specify shell environment variables and arguments passed to the program.

With the configuration above, the command `nicy run cheap-browser` replaces the script below :
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
One can easily substitute this script to the command `/usr/bin/cheap-browser`. For instance, if `$HOME/bin/nicy` folder have been added at the beginning of PATH environment variable, one can copy there the script into a new `cheap-browser` executable file and making it available as a replacement for the initial `nicy run cheap-browser` command.

With `nicy install` command, scripts for all the rules, after removing the duplicates and matching an existing command, are generated and installed in configured folder.

See also comments in the `environment` file.

## TODO
- Write a man page.
- Build a debian package.
- Add `set` subcommand to control running processes.
- Switch to python (the easy way) before reaching 2k LOC.

