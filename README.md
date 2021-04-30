# Nicy
## About
A bash script that limits the resources that a launched program and its spawned processes can share.
## Why
My legacy low-end hardware quickly turns nasty, running hot then shutting down, when launching too many or "resource hungry" softwares. But capping the resources that some programs request turns it useful again.

I used to install [Ananicy](https://github.com/Nefelim4ag/Ananicy), an auto nice daemon, with community rules support, that relies too on the Linux Control Groups ([Cgroups](https://en.wikipedia.org/wiki/Cgroups)).

I write nicy because I need to control the available resources per program according to some more context, thus adding options in a command line, not modifying a configuration file as a privileged user.

## Usage
```
$ nicy --dry-run nvim-qt
nicy: ulimit -S -e 23 &>/dev/null
nicy: systemctl --user start nicy-cpu66.slice &>/dev/null
nicy: systemctl --user --runtime set-property nicy-cpu66.slice CPUQuota=66% &>/dev/null
nicy: exec systemd-run --user -G -d --no-ask-password --quiet --unit=nvim-qt-13480 --scope --slice=nicy-cpu66.slice --nice=-3 -E 'SHELL=/bin/bash' /usr/bin/nvim-qt --nofork --nvim /usr/bin/nvim

$ nicy --help
Usage:
nicy [(-q|-v)|-n|-s] [-t TYPE|-d|-z] [-c CGROUP|--cpu{QUOTA}] [-u] [-m] CMD [ARGS]…
nicy --rebuild
nicy -k (rules|types|cgroups) [--from-dir=DIR]

Options:
  -q, --quiet          Be quiet and suppress additional informational output.
  -v, --verbose        Be verbose and display which command is launched.
  -n, --dry-run        Display commands but do not run them.
  -s, --build-script   Build a script that replaces the current commandline,
                       excluding the final arguments, and display it.

  -t, --type=TYPE      Control the set of properties applied. Use 'auto' to sea-
                       rch for the command rule set (default), 'cgroup-only' to
                       remove any property except the cgroup, 'default' or an
                       other defined type TYPE.
  -d, --default        Like '--type=default'. Do not search for a rule.
                       Apply the fallback values from the 'default' type.
  -z, --cgroup-only    Like '--type=cgroup-only'. Unset all other properties.

  -c, --cgroup=CGROUP  Run the command as part of this existing CGROUP.
      --cpu{QUOTA}     Like '--cgroup=cpu{QUOTA}' where QUOTA is an integer that
                       represents a percentage relative to the total CPU time
		       available on all cores.

  -m, --managed        Always run the command in a transient scope.
  -u, --force-cgroup   Run the command as part of an existing cgroup, if any,
                       that matches properties found, when no other cgroup has
                       been set.

      --rebuild        Rebuild the cache and exit.

  -k, --keys=KIND      List known KIND keys.
      --from-dir=DIR   Limit keys search to configuration files from DIR folder.

  -h, --help           Display this help and exit.
```

## Requirements
* [systemd](https://systemd.io/) and [systemd-run](https://www.freedesktop.org/software/systemd/man/systemd-run.html)
* [jq](https://stedolan.github.io/jq/), a lightweight and flexible command-line JSON processor
* [schedtool](https://github.com/freequaos/schedtool), a tool to change or query all CPU-scheduling policies

Most of cgroup settings are supported only if the unified control group hierarchy, the new version of kernel control group interface, see [Control Groups v2](https://www.kernel.org/doc/html/latest/admin-guide/cgroup-v2.html), is used. 
## Installation
Install as root in /usr/local.
`# make install`

Install as root in /usr.
`# make prefix=/usr install`
## Configuration
Nicy configuration is read from the following directories, in this order of preference :

- $XDG_CONFIG_HOME/nicy
- /etc/nicy
- /usr/local/etc/nicy

4 kinds of files are parsed :

- one **environment** file, at any directory root, to adjust default values
- at least one **.cgroups** file and one **.types** file, at any directory root
- none or many **.rules** files in any subdirectory

Moreover, when parsing **.cgroups**, **.types** and **.rules** [json](https://en.wikipedia.org/wiki/JSON) configuration files, path names are sorted by dictionary order to set priority.
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
For instance, rules in `temp.rules` have an higher priority than any rule from a file in `50-custom` directory, and types defined in `/home/user/.config/nicy` take precedence on any type from `/usr/local/etc/nicy`.

[Ananicy](https://github.com/Nefelim4ag/Ananicy) users can import their configuration files accordingly and start using nicy.
### .cgroups
>   A control group (abbreviated as [cgroup](https://en.wikipedia.org/wiki/Cgroups)) is a collection of processes that are bound by the same criteria and associated with a set of parameters or limits.

The .cgroups file lists in json format this set of parameters or limits for each cgroup. For instance :

`{ "cgroup": "cpu66", "CPUQuota": "66%" }`

 defines a **cpu66** cgroup, where the bound processes will share at maximum **66% of total CPU time available on ALL CORES** as per **CPUQuota** limit.

The *cgroup* pair is mandatory. All other pairs are optional and will be ignored if the key doesn't match the following parameters and limits :

- **CPUQuota**. Assign the specified CPU time quota to the processes executed. Takes a percentage value, suffixed with "%" or not. The percentage specifies how much CPU time the unit shall get at maximum, relative to the total CPU time available on ALL CORES.
- **MemoryHigh**. Specify the throttling limit on memory usage of the executed processes in this unit. Memory usage may go above the limit if unavoidable, but the processes are heavily slowed down and memory is taken away aggressively in such cases. This is the main mechanism to control memory usage of a unit. Takes a memory size in bytes. If the value is suffixed with K, M, G or T, the specified memory size is parsed as Kilobytes, Megabytes, Gigabytes, or Terabytes (with the base 1024), respectively. Alternatively, a percentage value may be specified, suffixed with "%" or not, which is taken relative to the installed physical memory on the system. If assigned the special value "infinity", no memory throttling is applied. **Requires the unified control group hierarchy.**
- **MemoryMax**. Specify the absolute limit on memory usage of the executed processes in this unit. If memory usage cannot be contained under the limit, out-of-memory killer is invoked inside the unit. It is recommended to use MemoryHigh= as the main control mechanism and use MemoryMax= as the last line of defense. Same format than MemoryHigh. **Requires the unified control group hierarchy.**
- **IOWeight**. Set the default overall block I/O weight for the executed processes. Takes a single weight value (between 1 and 10000) to set the default block I/O weight. This controls the "io.weight" control group attribute, which defaults to 100. For details about this control group attribute, see IO Interface Files[8]. The available I/O bandwidth is split up among all units within one slice relative to their block I/O weight. **Requires the unified control group hierarchy.**

Also see [systemd.resource-control(5)](https://www.freedesktop.org/software/systemd/man/systemd.resource-control.html).

Nicy relies on [systemd slice units](https://www.freedesktop.org/software/systemd/man/systemd.slice.html) to manage cgroups at run-time : a .cgroups file is required in order to declare available cgroups.
### .types
Writing a full rule for each program, out of many that nicy may run, would turn boring.

The .types file groups sets of properties in order to define program families. For instance, whether you run `nicy chromium` or `nicy firefox` you will probably set an higher scheduling priority for all processes, adjust the OOM score, and limit the available CPU time. So instead of writing twice, or more, the same rule with the same properties, you create a **type**.

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "CPUQuota": "66%" }`

or better, since you can run many programs inside the same existing cgroup :

`{ "type": "Web-Browser", "nice": -3, "oom_score_adj": 1000, "cgroup": "cpu66" }`

The *type* pair is mandatory. All other pairs are optional and will be ignored if the key doesn't match the above defined ones for .cgroups files or the following properties :

- **nice** ([renice(1)](https://manpages.debian.org/buster/bsdutils/renice.1.en.html)). Adjusted niceness, which affects process scheduling. Niceness values range from -20 (most favorable to the process) to 19 (least favorable to the process).
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
- **oom_score_adjust** ([choom(1)](https://manpages.debian.org/buster/schedtool/schedtool.8.en.html)). Out-Of-Memory killer score setting adjustement added to the badness score before it is used to determine which task to kill. Acceptable values range from -1000 to +1000.

Also see comments in the 00-types.types file.
### .rules
Once the available cgroups and types have been set, subdirectories can be filled with some .rules files and their many rules. For instance :
```
{ "name": "chromium", "type": "Web-Browser" }
{ "name": "firefox", "type": "Web-Browser" }
```
More properties can be added to the rule. For instance, any browser could quickly stands out as a memory hog if opening too many tabs, and it may be useful to limit the memory available for that program.
```
{ "name": "chromium", "type": "Web-Browser", "MemoryHigh": "60%", "MemoryMax": "75%" }
{ "name": "firefox", "type": "Web-Browser", "MemoryHigh": "60%", "MemoryMax": "75%" }
```
The *name* pair is mandatory. All other pairs are optional and will be ignored if the key doesn't match neither the pairs defined for .cgroups and .types files, nor *env* and *cmdargs* pairs, that respectively allow to specify environment variables and arguments passed to the command.

With the given configuration, the command `nicy firefox` almost replaces the script below :
```
#!/bin/bash
[ $UID -ne 0 ] && user_or_system=--user
ulimit -S -e 23 &>/dev/null
systemctl ${user_or_system} start nicy-cpu66.slice &>/dev/null
systemctl ${user_or_system} --runtime set-property nicy-cpu66.slice CPUQuota=66% &>/dev/null
exec systemd-run ${user_or_system} -G -d --no-ask-password --quiet \
    --unit=firefox-$$ --scope --slice=nicy-cpu66.slice \
    --nice=-3 -p MemoryHigh=60% -p MemoryMax=75% \
    choom -n 1000 -- `command -v firefox` "$@"
```
## nicy-path-helper
TODO.
## TODO
- Write a man page.
- Switch to python (the easy way).

