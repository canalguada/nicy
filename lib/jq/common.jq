module { "name": "common" };

# One-liners and small functions
# ==============================

def within($array): . as $element | any($array[]; . == $element) ;

def object_to_array: to_entries | map("\(.key)=\(.value)"|@sh) ;

def object_to_string: object_to_array | join(" ") ;

def array_to_string: map("\(.)"|@sh) | join(" ") ;

def is_percentage: tostring | test("^[1-9][0-9]?%?$"; "") ;

def is_bytes: tostring | test("^[1-9][0-9]+(K|M|G|T)?$"; "i") ;

def in_range($from; $upto): tonumber | . >= $from and . <= $upto ;


# Parsing cgroups, types and rules
# ================================

# Nicy quota value is a percentage of total CPU time for ALL cores
def cpuquota_adjust($cores): "\((rtrimstr("%") | tonumber) * $cores)%" ;

def check_memory_value:
  if .value == "infinity" then .
  elif .value | is_percentage then .value |= "\(rtrimstr("%"))%"
  elif .value | is_bytes then .value |= ascii_upcase
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_cpuquota:
  if .value | is_percentage then .key |= "CPUQuota"
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_ioweight:
  if .value | in_range(1; 10000) then .key |= "IOWeight"
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_memoryhigh: .key |= "MemoryHigh" | check_memory_value ;

def parse_memorymax: .key |= "MemoryMax" | check_memory_value ;

def parse_nice:
  if .value | in_range(-20; 19) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_sched:
  if .value | within(["other", "fifo", "rr", "batch", "idle"]) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_rtprio:
  if .value | in_range(1; 99) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_ioclass:
  if .value | within(["none", "realtime", "best-effort", "idle"]) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_ionice:
  if .value | in_range(0; 7) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_oom_score_adj:
  if .value | in_range(-1000; 1000) then .
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_cmdargs:
  if (.value | type) == "array" then .
  elif (.value | type) == "string" then .value |= [.value]
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

def parse_env:
  if (.value | type) == "object" then .value |= object_to_array
  elif (.value | type) == "array" then .
  elif (.value | type) == "string" then .value |= [.value]
  else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

# Using cache objects
# ===================

def cgroups: $cachedb.cgroups ;

def types: $cachedb.types ;

def rules: $cachedb.rules ;

def rule_names: rules | map("\(.name)") | unique ;

def get_rule($value): first(rules[] | select(.name == $value)) ;

def get_type_or_die($value):
  first(types[] | select(.type == $value))? // {}
  | if length == 0 then
    error("unknown type '\($value)'\n")
  else . end ;

def get_cgroup_or_die($value):
  first(cgroups[] | select(.cgroup == $value))? // {}
  | if length == 0 then
    error("unknown cgroup '\($value)'\n")
  else . end ;

def available_in_slice: ["CPUQuota", "IOWeight", "MemoryHigh", "MemoryMax"] ;

def cgroup_keys: ["cgroup"] + available_in_slice ;

def type_keys:
  ["type"]
  + ["nice", "sched", "rtprio", "ioclass", "ionice", "oom_score_adj"]
  + cgroup_keys ;

def rule_keys: ["name"] + type_keys + ["cmdargs", "env"] ;

def cgroup_paths: reduce cgroup_keys[] as $key ([]; . + [$key]) ;

def cgroup_only_keys: rule_keys - type_keys + ["cgroup"] ;


# Building `get_entries` input
# ===================================
# {
#   "request": .,
#   "entries": {
#     "cred": []
#   },
#   "diff": {},
#   "commands": []
# }

def fake_values($shell; $nproc; $max_nice):
  {
    "name": .,
    "cmd": "%\(.)%",
    "preset": "auto",
    "cgroup": "",
    "probe_cgroup": false,
    "managed": false,
    "quiet": true,
    "verbosity": 0,
    "shell": $shell,
    "nproc": $nproc,
    "max_nice": $max_nice,
    "cpusched": "0:other:0",
    "iosched": "0:none:0"
  } ;

def get_input:
  {
    "request": .,
    "entries": {
      "cred": []
    },
    "diff": {},
    "commands": []
  } ;


# Filtering `get_entries` input
# =============================

def rule_or_default: get_rule(.request.name)? // get_type_or_die("default") ;

def type_or_die: get_type_or_die(.request.preset) ;

def has_entry($p): .entries | has($p) ;

def _($p): .entries."\($p)" ;

def del_entry(f):
  if f|type == "string" then del(.entries."\(f)")
  else del(.entries."\(f[])") end ;

def keep_cgroup_only: del_entry((.entries|keys) - cgroup_only_keys) ;

def move_to_cgroup: .entries += get_cgroup_or_die(.request.cgroup) ;

def current_cgroup_properties: .entries | delpaths([paths] - cgroup_paths) ;

def slice_unit: "nicy-\(_("cgroup")).slice" ;

def format_slice_properties:
  (get_cgroup_or_die(_("cgroup")) | del(.cgroup, .origin)) as $properties
  | .request.nproc as $cores
  | del_entry($properties | keys)
  | .entries += {
    "slice_properties": (
      $properties
      | if has("CPUQuota") then .CPUQuota |= cpuquota_adjust($cores)
      else . end
      | to_entries
      | map("\(.key)=\(.value)")
      | join(" ")
    )
  } ;

def best_matching($key; $object):
  [.[] | select(del(.[$key]) | inside($object))]
  | sort_by(length)
  | reverse
  | first(.[] | .[$key])? // null ;

def move_to_matching_cgroup:
  .entries += {
    "cgroup": (
      current_cgroup_properties as $properties
      | if ($properties | length) == 0 then null
      else cgroups | best_matching("cgroup"; $properties) end
    )
  } ;

def rule_or_type:
  if .request.preset | within(["auto", "cgroup-only"])
  then rule_or_default
  else type_or_die end ;

def load_from($rule):
  def type_entry:
    if $rule | has("type") then get_type_or_die($rule.type)
    else null end ;
  def cgroup_entry:
    if $rule | has("cgroup") then get_cgroup_or_die($rule.cgroup)
    else null end ;

  type_entry as $rtype
  | cgroup_entry as $rcgroup
  | .entries += $rtype + $rcgroup + ($rule | del(.type, .cgroup)) ;

def set_credentials:
  .entries |= (
    if (has("nice") and (.nice < 0)) then .cred += ["nice"]
    else . end
    | if (has("sched") and ((.sched == "fifo") or (.sched == "rr"))) then
      .cred += ["sched"]
    else . end
    | if (has("ioclass") and (.ioclass == "realtime")) then
      .cred += ["ioclass"]
    else . end
  ) ;

def sudo_is_set: .entries.cred | contains(["sudo"]) ;

def set_sudo: .entries.cred += ["sudo"] ;


# Use 'auto' to get rule for 'name' command, if any (default).
# Use 'cgroup-only' to remove everything but cgroup from rule.
# Use 'default' or any other type.
def get_entries:
  load_from(rule_or_type)
  | if .request.preset == "cgroup-only" then keep_cgroup_only
  else . end
  # | if .request.cgroup != null then move_to_cgroup else . end
  | if (.request.cgroup | length) > 0 then move_to_cgroup
  else . end
  # No cgroup defined in configuration files nor requested by user
  | if (has_entry("cgroup") | not) and (.request.probe_cgroup == true) then
      move_to_matching_cgroup
  # Use cgroup from rule found in configuration file, if any
  else . end
  | del_entry(["name", "type"])
  # Extract and format slice properties, if any
  # | if _("cgroup") != null then format_slice_properties
  | if (_("cgroup") | length) > 0 then format_slice_properties
  else del_entry("cgroup") end
  | set_credentials
  ;


# Filtering `get_commands` input
# =============================

# Managing commands
# =================

def add_command($cmdargs): .commands += [$cmdargs] ;

def quiet_command($cmdargs): add_command($cmdargs + [">/dev/null"]) ;

def append_to_command($cmdargs):
  if (.commands | length) > 0 then .commands[-1] += $cmdargs
  else add_command($cmdargs) end ;

def append_to_exec_command($cmdargs): .exec_cmd += $cmdargs ;

def set_sudo_if_required:
  if sudo_is_set | not then
    # Require authentication for privileged operations.
    add_command(["[ $(id -u) -ne 0 ] && SUDO=\(env.SUDO) || SUDO="])
    | set_sudo
  else . end ;

def cmd_start_slice_unit($user_or_system):
  quiet_command(["systemctl", $user_or_system, "start", slice_unit]) ;

def cmd_set_slice_properties($user_or_system):
  quiet_command([
    "systemctl", $user_or_system, "--runtime",
    "set-property", slice_unit, _("slice_properties")
  ]) ;


# System utilities command args per pid and pgrp, when available
# ==============================================================

# $prio parameter is `nice` value in type/rule
def renice($prio; $pid; $pgrp):
  ["renice", "-n", "\($prio)"]
  | if ($pid != null) and ($pgrp == null) then . + ["-p", "\($pid)"]
  elif ($pid == null) and ($pgrp != null) then . + ["-g", "\($pgrp)"]
  elif ($pid != null) and ($pgrp != null) then
    error("can set niceness for both pid and pgrp.")
  # Require running processes
  else error("need a pid or a pgrp") end ;

# $oom_score_adj is `oom_score_adj` value in type/rule
def choom($oom_score_adj; $pid):
  ["choom", "-n", "\($oom_score_adj)"]
  | if $pid != null then . + ["-p", "\($pid)"]
  # Set oom score adjust of the next command
  else . + ["--"] end ;

# def schedtool($policy; $prio; $pid):
#   [ "schedtool" ]
#   | {
#     "other": "-N",
#     "fifo": "-F",
#     "rr": "-R",
#     "batch": "-B",
#     "idle": "-D"
#   } as $policy_map
#   | if $policy != null then . + [$policy_map[$policy]] else . end
#   | if $prio != null then . + ["-p", "\($prio)"] else . end
#   | if $pid != null then . + ["\($pid)"]
#   # Set policy and priority of the next command
#   else . + ["-e"] end ;

# $policy is `sched` value in type/rule
# $prio is `rtprio` value in type/rule
def chrt($policy; $prio; $pid):
  [ "chrt" ]
  | {
    "other": "--other",
    "fifo": "--fifo",
    "rr": "--rr",
    "batch": "--batch",
    "idle": "--idle"
  } as $policy_map
  | if $policy != null then . + [$policy_map[$policy]] else . end
  | if $pid != null then
    . + ["-a", "-p", "\(if $prio != null then $prio else 0 end)", "\($pid)"]
  # Set policy and priority of the next command
  else . + ["\(if $prio != null then $prio else 0 end)"] end ;

# $class is `ioclass` value in type/rule
# $level is `ionice` value in type/rule
def ionice($class; $level; $pid; $pgrp):
  [ "ionice" ]
  | {
    "none": "0",
    "realtime": "1",
    "best-effort": "2",
    "idle": "3"
  } as $class_map
  # If no class is specified, then command will be executed  with the
  # "best-effort" scheduling class.  The default priority level is 4.
  | if $class != null then . + ["-c", $class_map[$class]] else . end
  | if $level != null then . + ["-n", "\($level)"] else . end
  | if ($pid != null) and ($pgrp == null) then . + ["-p", "\($pid)"]
  elif ($pid == null) and ($pgrp != null) then . + ["-P", "\($pgrp)"]
  elif ($pid != null) and ($pgrp != null) then
    error("can set io scheduling class for both pid and pgrp.")
  # Set class and priority of the next command
  else . end ;


# Filtering `get_commands` input
# ==============================
# {
#   "request": {
#     ...
#   },
#   "entries": {
#     "cred": [
#       ...
#     ],
#     ...
#   },
#   "diff": {},
#   "commands": [],
#   "use_scope": false,
#   "exec_cmd": []
# }

def process_nice($pid; $pgrp):
  def need_sudo: .entries.cred | contains(["nice"]) ;
  def ulimit: ["ulimit", "-S", "-e", "\(20 - _("nice"))"] ;
  def supported_shell:
    (.request.shell | sub(".*/"; ""; "l")) as $basename
    | ($basename == "bash") or ($basename == "zsh") ;

  if has_entry("nice") then
    if need_sudo then
      if has("use_scope") and .use_scope
        and (20 - _("nice") <= .request.max_nice)
        and supported_shell then
        # Update soft limit and let systemd-run change the niceness
        quiet_command(ulimit)
      else
        set_sudo_if_required
        | quiet_command(["$SUDO"] + renice(_("nice"); $pid; $pgrp))
        | del_entry("nice")
      end
    else
        quiet_command(renice(_("nice"); $pid; $pgrp))
        | del_entry("nice")
    end
  else . end ;

def process_oom_score_adj($pid):
  if has_entry("oom_score_adj") then
    # Always set oom score adjust outside systemd-run command
    quiet_command(choom(_("oom_score_adj"); $pid))
    | del_entry("oom_score_adj")
  else . end ;

def process_sched_rtprio($pid):
  def need_sudo: .entries.cred | contains(["sched"]) ;
  # setting fifo or rr requires priority value
  def has_prio: .request.cpusched / ":" | (.[0] == "1") or (.[0] == "2") ;
  def require_prio: has("sched") and ((.sched == "fifo") or (.sched == "rr")) ;

  has_prio as $has_prio
  | (
    .entries
    | [(.sched)? // null,
      if has("rtprio") then
        # Validate that setting rtprio could make sense
        if require_prio or ((has("sched") | not) and $has_prio)
        then .rtprio
        else 0 end
      else null end
    ]
  ) as $args
  | del_entry(["sched", "rtprio"])
  | if $args != [null, null] then
    # Set privileges when required
    if need_sudo then
      set_sudo_if_required
      | quiet_command(["$SUDO"] + ($args | chrt(.[0]; .[1]; $pid)))
    else
      quiet_command($args | chrt(.[0]; .[1]; $pid))
    end
  else . end ;

def process_ioclass_ionice($pid; $pgrp):
  def need_sudo: .entries.cred | contains(["ioclass"]) ;
  # setting realtime or best-effort requires priority value
  def has_prio: .request.iosched / ":" | (.[0] == "1") or (.[0] == "2") ;
  def require_prio:
    has("ioclass") and (
      (.ioclass == "realtime") or (.ioclass == "best-effort")
    ) ;

  has_prio as $has_prio
  | (
    .entries
    | [
      (.ioclass)? // null,
      if has("ionice") then
        if require_prio or ((has("ioclass") | not) and $has_prio) then
          .ionice
        else null end
      else null end
    ]
  ) as $args
  | del_entry(["ioclass", "ionice"])
  | if $args != [null, null] then
    if need_sudo then
      set_sudo_if_required
      | quiet_command(["$SUDO"] + ($args | ionice(.[0]; .[1]; $pid; $pgrp)))
    else
      quiet_command($args | ionice(.[0]; .[1]; $pid; $pgrp))
    end
  else . end ;

def process_env:
  if has_entry("env") then
    if .use_scope then .
    else
      add_command(["export"] + (_("env") | object_to_array))
      | del_entry("env")
    end
  else . end ;

def process_cmdargs:
  .exec_cmd += [.request.cmd]
  | if has_entry("cmdargs") then
    .exec_cmd += _("cmdargs")
    | del_entry("cmdargs")
  else . end ;

def get_commands:
  def process_exec:
    def quiet_or_not:
      if .request.quiet or (.request.verbosity < 2) then "--quiet"
      else "" end ;
    def unit: "--unit=\(.request.name)-$$" ;

    # Use systemd scope unit
    if .use_scope then
      # Run inside some cgroup
      if has_entry("cgroup") then
        # Start slice
        cmd_start_slice_unit("${user_or_system}")
        # Set properties
        | if has_entry("slice_properties") then
          cmd_set_slice_properties("${user_or_system}")
          | del_entry("slice_properties")
        else . end
      else . end
      # Build command
      | add_command([
        "exec", "systemd-run", "${user_or_system}", "-G", "-d",
        "--no-ask-password", quiet_or_not, unit, "--scope"
      ])
      | if has_entry("cgroup") then
        append_to_command(["--slice=\(slice_unit)"])
        | del_entry("cgroup")
      else . end
      # Adjust scope properties, thus outside cgroup/slice
      | if has_entry("CPUQuota") then
        _("CPUQuota") |= cpuquota_adjust(.request.nproc)
      else . end
      | reduce available_in_slice[] as $property (
        . ;
        if has_entry($property) then
          append_to_command(["-p", "\($property)=\(_($property))"])
          | del_entry($property)
        else . end
      )
      | if has_entry("nice") then
        append_to_command(["--nice=\(_("nice"))"])
        | del_entry("nice")
      else . end
      | if has_entry("env") then
        append_to_command((_("env") | object_to_array | map("-E", "\(.)")))
        | del_entry("env")
      else . end
    # No scope unit required
    else add_command(["exec"]) end
    # Finally append exec command
    | append_to_command(.exec_cmd) ;

  . + {
    "use_scope": false,
    "exec_cmd": []
  }
  | if .request.managed or (has_entry("cgroup")) then .use_scope = true
  else . end
  | if .use_scope then
    add_command([
      "[ $(id -u) -ne 0 ] && user_or_system=--user || user_or_system="
    ])
  else . end
  | process_env
  | process_nice("$$"; null)
  | process_oom_score_adj("$$")
  | process_sched_rtprio("$$")
  | process_ioclass_ionice("$$"; null)
  | process_cmdargs
  | process_exec ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
