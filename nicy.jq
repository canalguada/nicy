module "nicy" ;

# Common functions
# ================

def jsonify:
  if test("^[0-9]*$"; "") then tonumber
  elif test("^true$"; "") then true
  elif test("^false$"; "") then false
  elif test("^null$"; "") then null
  else tostring
  end ;


def _contains($subarray): indices($subarray) | length > 0 ;


def _inside($array): . as $subarray | $array | _contains($subarray) ;


# def to_object($in_array):
#   $in_array
#   | reduce range(0; length; 2) as $i (
#     {}; . += { "\($in_array[$i])": $in_array[$i + 1] }
#   ) ;


def cgroups: $cachedb.cgroups ;


def types: $cachedb.types ;


def rules: $cachedb.rules ;


def get_rule($value): first(rules[] | select(.name == $value)) ;


def get_type_or_die($value):
  first(types[] | select(.type == $value))? // {}
  | if length == 0 then
    "unknown type '\($value)'\n"|halt_error(2)
  else . end ;


def get_cgroup_or_die($value):
  first(cgroups[] | select(.cgroup == $value))? // {}
  | if length == 0 then
    "unknown cgroup '\($value)'\n"|halt_error(4)
  else . end ;


def available_in_slice: ["CPUQuota", "IOWeight", "MemoryHigh", "MemoryMax"] ;


def cgroup_keys: ["cgroup"] + available_in_slice ;


def type_keys:
  ["type"]
  + ["nice", "sched", "rtprio", "ioclass", "ionice", "oom_score_adj"]
  + cgroup_keys ;


def rule_keys: ["name"] + type_keys + ["cmdargs", "env"] ;


def cgroup_paths: reduce cgroup_keys[] as $key ([]; . + [$key]) ;


def is_percentage: tostring | test("^[1-9][0-9]?%?$"; "") ;


def is_bytes: tostring | test("^[1-9][0-9]+(K|M|G|T)?$"; "i") ;


def in_range($from; $upto): tonumber | . >= $from and . <= $upto ;


def in_array($content): [.] | inside($content) ;


def object_to_array: to_entries | map("\(.key)=\(.value)"|@sh) ;


def object_to_string: object_to_array | join(" ") ;


def array_to_string: map("\(.)"|@sh) | join(" ") ;


def best_matching($key; $object):
  [.[] | select(del(.[$key]) | inside($object))]
  | sort_by(length)
  | reverse
  | first(.[] | .[$key])? // null ;


# Nicy quota value is a percentage of total CPU time for ALL cores
def cpuquota_adjust($cores): "\((rtrimstr("%") | tonumber) * $cores)%" ;


def dump($kind):
  [ $cachedb."\($kind)s"[]
    |select(.origin|in_array($ARGS.positional))
  ] | (if $kind == "rule" then "name" else "\($kind)" end) as $key
  | unique_by(."\($key)")
  | map([."\($key)", "\(del(.origin))"]) as $rows
  | ["\($key)", "content"] as $cols
  | $cols, $rows[]
  | @tsv ;


# Main filter
# ===========

# get_entries input filter
# ------------------------
# {
#   "request": ($ARGS.positional | build_request),
#   "entries": {
#     "cred": []
#   }
# }

# Managing entries
def has_entry($p): .entries | has($p) ;


def _($p): .entries."\($p)" ;


def del_entry(f):
  if f|type == "string" then del(.entries."\(f)")
  else del(.entries."\(f[])") end ;


def request_template:
  {
    "name": null,
    "cmd": null,
    "preset": null,
    "cgroup": null,
    "probe_cgroup": null,
    "managed": null,
    "quiet": null,
    "verbosity": null,
    "shell": null,
    "nproc": null,
    "max_nice": null,
    "policies": {
      "sched": null,
      "io": null
    }
  } ;


def build_request:
  map(jsonify) as $jsonargs
  | request_template
  | ([paths] - [["policies"]]) as $paths
  | last(
    foreach range(0; length + 1) as $pos (
      {};
      setpath($paths[$pos]; $jsonargs[$pos])
    )
  ) ;


# Use 'auto' option to get the rule for the 'name' command, if any (default).
# Use 'cgroup-only' to remove everything but the cgroup from the rule.
# Use 'default' or an other defined type.
def get_entries:
  def rule_or_default: get_rule(.request.name)? // get_type_or_die("default") ;

  def type_or_die: get_type_or_die(.request.preset) ;

  def cgroup_only_keys: rule_keys - type_keys + ["cgroup"] ;

  def keep_cgroup_only: del_entry((.entries|keys) - cgroup_only_keys) ;

  def move_to_cgroup: .entries += get_cgroup_or_die(.request.cgroup) ;

  def current_cgroup_properties: .entries | delpaths([paths] - cgroup_paths) ;

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

  def move_to_matching_cgroup:
    .entries += {
      "cgroup": (
        current_cgroup_properties as $properties
        | if ($properties | length) == 0 then null
        else cgroups | best_matching("cgroup"; $properties) end
      )
    } ;

  def rule_or_type:
    if .request.preset | in_array(["auto", "cgroup-only"])
    then rule_or_default
    else type_or_die
    end ;

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
      if (has("nice") and (.nice < 0)) then .cred += ["nice"] else . end
      | if (has("sched") and (.sched | in_array(["fifo", "rr"]))) then
        .cred += ["sched"]
      else . end
      | if (has("ioclass") and (.ioclass == "realtime")) then
        .cred += ["ioclass"]
      else . end
    ) ;

  load_from(rule_or_type)
  | if .request.preset == "cgroup-only" then keep_cgroup_only else . end
  | if .request.cgroup != null then move_to_cgroup else . end
  # No cgroup defined in configuration files nor requested by user
  | if (has_entry("cgroup") | not) and (.request.probe_cgroup == true) then
      move_to_matching_cgroup
  # Use cgroup from rule found in configuration file, if any
  else . end
  | del_entry(["name", "type"])
  | if _("cgroup") != null then
    # Extract and format slice properties, if any
    format_slice_properties
  else del_entry("cgroup") end
  | set_credentials ;


# get_commands input filter
# ------------------------
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
#   "use_scope": false,
#   "commands": [],
#   "exec_cmd": []
# }

# Managing entries and commands

def add_command($cmdargs): .commands += [$cmdargs] ;


def quiet_command($cmdargs): add_command($cmdargs + [">/dev/null"]) ;


def append_to_command($cmdargs):
  if (.commands | length) > 0 then
    .commands[-1] += $cmdargs
  else add_command($cmdargs) end ;


def set_sudo_if_required:
  def sudo_is_set: .entries.cred | contains(["sudo"]) ;

  def set_sudo: .entries.cred += ["sudo"] ;

  if sudo_is_set | not then
    # Require authentication for privileged operations.
    add_command(["[ $(id -u) -ne 0 ] && SUDO=\(env.SUDO) || SUDO="])
    | set_sudo
  else . end ;


def renice($prio; $pid; $pgrp):
  ["renice", "-n", "\($prio)"]
  | if ($pid != null) and ($pgrp == null) then . + ["-p", "\($pid)"]
  elif ($pid == null) and ($pgrp != null) then . + ["-g", "\($pgrp)"]
  elif ($pid != null) and ($pgrp != null) then
    error("can set niceness for both pid and pgrp.")
  # Require running processes
  else error("need a pid or a pgrp") end ;


def choom($oom_score_adj; $pid):
  ["choom", "-n", "\($oom_score_adj)"]
  | if $pid != null then . + ["-p", "\($pid)"]
  # Set oom score adjust of the next command
  else . + ["--"] end ;


def schedtool($policy; $prio; $pid):
  [ "schedtool" ]
  | {
    "other": "-N",
    "fifo": "-F",
    "rr": "-R",
    "batch": "-B",
    "idle": "-D"
  } as $policy_map
  | if $policy != null then . + [$policy_map[$policy]] else . end
  | if $prio != null then . + ["-p", "\($prio)"] else . end
  | if $pid != null then . + ["\($pid)"]
  # Set policy and priority of the next command
  else . + ["-e"] end ;


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
  | . + ["-a"]
  | if $pid != null then
    . + ["-p", "\(if $prio != null then $prio else 0 end)", "\($pid)"]
  # Set policy and priority of the next command
  else . + ["\(if $prio != null then $prio else 0 end)"]
  end ;


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
  | if ($pid != null) and ($pgrp == null) then
    . + ["-p", "\($pid)"]
  elif ($pid == null) and ($pgrp != null) then
    . + ["-P", "\($pgrp)"]
  elif ($pid != null) and ($pgrp != null) then
    error("can set io scheduling class for both pid and pgrp.")
  # Set class and priority of the next command
  else . end ;


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
    elif .use_scope | not then
        quiet_command(renice(_("nice"); $pid; $pgrp))
        | del_entry("nice")
    else . end
  else . end ;


def process_oom_score_adj($pid):
  if has_entry("oom_score_adj") then
    if has("use_scope") and .use_scope then
      .exec_cmd += choom(_("oom_score_adj"); null)
    else
      quiet_command(choom(_("oom_score_adj"); $pid))
    end
    | del_entry("oom_score_adj")
  else . end ;


def process_sched_rtprio($pid):
  def need_sudo: .entries.cred | contains(["sched"]) ;

  def running_rt: .request.policies.sched | test("SCHED_(RR|FIFO)"; "") ;

  def rt_required: has("sched") and ((.sched == "fifo") or (.sched == "rr")) ;

  running_rt as $running_rt
  | (
    .entries
    | [(.sched)? // null,
      if has("rtprio") then
        # Validate that setting rtprio could make sense
        if rt_required or ((has("sched") | not) and $running_rt)
        then .rtprio
        else 0 end
      else null end
    ]
  ) as $args
  | del_entry(["sched", "rtprio"])
  | if $args != [null, null] then
    if has("use_scope") and .use_scope then
      .exec_cmd += ($args | chrt(.[0]; .[1]; null))
    else
      # Set privileges when required
      if need_sudo then
        set_sudo_if_required
        | quiet_command(["$SUDO"] + ($args | chrt(.[0]; .[1]; $pid)))
      else
        quiet_command($args | chrt(.[0]; .[1]; $pid))
      end
    end
  else . end ;


def process_ioclass_ionice($pid; $pgrp):
  def need_sudo: .entries.cred | contains(["ioclass"]) ;

  def has_prio: test("(realtime|best-effort)"; "") ;

  def require_prio: has("ioclass") and (.ioclass | has_prio) ;

  (.request.policies.io | has_prio) as $has_prio
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
    if has("use_scope") and .use_scope then
      .exec_cmd += ($args | ionice(.[0]; .[1]; null; null))
    else
      if need_sudo then
        set_sudo_if_required
        | quiet_command(["$SUDO"] + ($args | ionice(.[0]; .[1]; $pid; $pgrp)))
      else
        quiet_command($args | ionice(.[0]; .[1]; $pid; $pgrp))
      end
    end
  else . end ;


def slice_unit: "nicy-\(_("cgroup")).slice" ;


def cmd_start_slice_unit($user_or_system):
  quiet_command(["systemctl", $user_or_system, "start", slice_unit]) ;


def cmd_set_slice_properties($user_or_system):
  quiet_command([
    "systemctl", $user_or_system, "--runtime",
    "set-property", slice_unit, _("slice_properties")
  ]) ;


def get_commands:
  def process_env:
    if has_entry("env") then
      if .use_scope then .
      else
        add_command(["export"] + _("env"))
        | del_entry("env")
      end
    else . end ;

  def process_exec:
    def build($property):
      .entries
      | if has($property) then
        ."\($property)" as $value
        | delpaths([[$property]])
        | [ "-p", "\($property)=\($value)" ]
      else null end ;

    def prepare_scope_args:
      .exec_args += [
        "systemd-run", "${user_or_system}", "-G", "-d", "--no-ask-password"
      ]
      | if .request.quiet or (.request.verbosity < 2) then
        .exec_args += ["--quiet"]
      else . end
      | .request.name as $name
      | .exec_args += ["--unit=\($name)-$$", "--scope"] ;

    . + { "exec_args": [] }
    # Use systemd scope unit
    | if .use_scope then
      prepare_scope_args
      # Run inside some cgroup
      | if has_entry("cgroup") then
        # Start slice
        cmd_start_slice_unit("${user_or_system}")
        # Set properties
        | if has_entry("slice_properties") then
          cmd_set_slice_properties("${user_or_system}")
          | del_entry("slice_properties")
        else . end
        | .exec_args += ["--slice=\(slice_unit)"]
        | del_entry("cgroup")
      else . end
      # Adjust scope properties, thus outside cgroup/slice
      | if has_entry("CPUQuota") then
        _("CPUQuota") |= cpuquota_adjust(.request.nproc)
      else . end
      | reduce available_in_slice[] as $property (
        . ; .exec_args += build($property)
      )
      | if has_entry("nice") then
        .exec_args += [ "--nice=\(_("nice"))" ]
        | del_entry("nice")
      else . end
      | if has_entry("env") then
        .exec_args += (_("env") | map("-E", "\(.)"))
        | del_entry("env")
      else . end
    else . end
    | .exec_cmd += [.request.cmd]
    | if has_entry("cmdargs") then
      .exec_cmd += _("cmdargs")
      | del_entry("cmdargs")
    else . end
    # Finally build command
    | add_command(["exec"] + .exec_args + .exec_cmd) ;

  . + {
    "use_scope": false,
    "commands": [],
    "exec_cmd": []
  }
  | if .request.managed or (has_entry("cgroup")) then
    .use_scope = true
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
  | process_exec ;


def main:
  # Expects request values as positional args and cachedb as named json arg
  { "request": ($ARGS.positional | build_request), "entries": { "cred": [] } }
  | get_entries
  | get_commands
  | if has("error") then
     "error", .error
   else "commands", (.commands[] | (map(@sh) | join(" ")))
  end ;


# Parsing functions
# =================

# Validate entry, ignore unknown keys or raise error on bad value
def parse_entry($kind):
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
    if .value | in_array(["other", "fifo", "rr", "batch", "idle"]) then .
    else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

  def parse_rtprio:
    if .value | in_range(1; 99) then .
    else error("while parsing '\(.key)' key, bad value : \(.value)") end ;

  def parse_ioclass:
    if .value | in_array(["none", "realtime", "best-effort", "idle"]) then .
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

  label $out
  | (
    if $kind == "rule" then rule_keys + ["origin", "type", "cgroup"]
    elif $kind == "type" then type_keys + ["origin", "cgroup"]
    elif $kind == "cgroup" then cgroup_keys + ["origin"]
    else empty end
  ) as $keys
  | .key |= ascii_downcase
  | if .key | in_array($keys | map(ascii_downcase)) | not then break $out
  else . end
  | if .key | in_array(["origin", "name", "type","cgroup"]) then .
  elif .key | in_array(cgroup_keys | map(ascii_downcase)) then
    if .key == "cpuquota" then parse_cpuquota
    elif .key == "ioweight" then parse_ioweight
    elif .key == "memoryhigh" then parse_memoryhigh
    elif .key == "memorymax" then parse_memorymax
    else break $out end
  elif .key | in_array(type_keys - cgroup_keys) then
    if .key == "nice" then parse_nice
    elif .key == "sched" then parse_sched
    elif .key == "rtprio" then parse_rtprio
    elif .key == "ioclass" then parse_ioclass
    elif .key == "ionice" then parse_ionice
    elif .key == "oom_score_adj" then parse_oom_score_adj
    else break $out end
  elif .key | in_array(rule_keys - type_keys) then
    if .key == "cmdargs" then parse_cmdargs
    elif .key == "env" then parse_env
    else break $out end
  else break $out end ;


def make_cache($kind):
  (if $kind == "rule" then "name" else $kind end) as $key
  # Keep all objects in order to filter per origin later, if required
  # When parsing, only remove optional entries
  | unique_by(.origin, ."\($key)")
  | .[]
  | if type == "object" then with_entries(parse_entry($kind))
  # Raise error
  else ("cannot parse '\(type)' input building cache" | halt_error(1)) end ;


# Install functions
# =================

def rule_names: rules | map("\(.name)") | unique ;


def rule_request_values($shell; $nproc; $max_nice):
  [
    .,
    "%\(.)%",
    "auto",
    null,
    false,
    false,
    true,
    0,
    $shell,
    $nproc,
    $max_nice,
    "PID  0: PRIO   0, POLICY N: SCHED_NORMAL  , NICE   0, AFFINITY 0x1",
    "none: prio 0"
  ] ;


def stream_scripts_from_rules($shell; $nproc; $max_nice):
  def names: rule_names | . - $ARGS.positional ;

  def values: rule_request_values($shell; $nproc; $max_nice) ;

  names[]
  | {
    "request": (values | map(tostring) | build_request),
    "entries": {
      "cred": []
    }
  }
  | get_entries
  | get_commands
  | if has("error") then
     "error", .error
  else
    "begin \(.request.name)",
    "'#!\(.request.shell)'",
    (.commands[] | (map(@sh) | join(" "))),
    "end \(.request.name)"
  end ;


# Runtime functions
# =================

def get_runtime_entries:
  rule_names as $names
  | .[]
  # Filter comm without rule
  # TODO: Find better test
  | select([.comm] | _inside($names))
  # Raw pid status entries
  | .
  + { "sched": ["other", "fifo", "rr", "batch", "iso", "idle"][.policy] }
  + (
    .comm
    | {
      "request": (
        rule_request_values($shell; $nproc; $max_nice)
        | map(tostring)
        | build_request
      ),
      "entries": {
        "cred": []
      },
      "diff": {}
    }
  )
  | get_entries
  | reduce type_keys[] as $key (
    . ;
    if (.entries | has($key)) and (."\($key)" != .entries."\($key)") then
      .diff += { "\($key)": .entries."\($key)" }
    else .  end
  )
  | if .diff | has("cgroup") then
    .diff.cgroup as $cgroup
    # Running yet inside its nicy cgroup
    | if (.cgroup | test("nicy.slice/nicy-\($cgroup).slice"; "")) then
      del(.diff.cgroup)
    else . end
  else . end ;


def stream_runtime_adjust:
  def in_session_scope: test("user-[0-9]+.slice/session-[0-9]+.scope"; "") ;

  def in_app_slice: test("user@[0-9]+.service/app.slice"; "") ;

  def in_own_app_slice: test("user@\($uid).service/app.slice"; "") ;

  def in_system_slice: test("system.slice"; "") ;

  def unit: .cgroup | sub(".*/"; "") ;

  def adjust_properties($pgrp):
    def adjust_per_pid($pid):
      .pids."\($pid)".diff as $diff
      | if $diff | has("oom_score_adj") then
        .pids."\($pid)" |= process_oom_score_adj(.pid)
      else . end
      | if $diff | (has("sched") or has("rtprio")) then
        .pids."\($pid)" |= process_sched_rtprio(.pid)
      else . end ;

    def remove_diff_entries($pid):
      del(.pids."\($pid)".diff."\((type_keys - cgroup_keys)[])") ;

    .pgrps."\($pgrp)"[0] as $lpid
    | if .pids."\($lpid)".diff | has("nice") then
      # renice can set niceness per process group
      .pids."\($lpid)" |= process_nice(null; $pgrp)
    else . end
    | if .pids."\($lpid)".diff | (has("ioclass") or has("ionice")) then
      # ionice can set io scheduling class and priority per process group
      .pids."\($lpid)" |= process_ioclass_ionice(null; $pgrp)
    else . end
    | reduce .pgrps."\($pgrp)"[] as $pid (
      . ;
      adjust_per_pid($pid)
      | remove_diff_entries($pid)
    ) ;

  def start_slice_unit($lpid):
    # Start slice
    .pids."\($lpid)" |= cmd_start_slice_unit("--user")
    # Set properties
    | if .pids."\($lpid)" | has_entry("slice_properties") then
      .pids."\($lpid)" |= cmd_set_slice_properties("--user")
    else . end ;

  def has_key($entry): . as $i | $entry | has($i) ;

  def require_scope($pgrp):
    .pids."\(.pgrps."\($pgrp)"[0])".diff as $diff
    | any(cgroup_keys|.[]; has_key($diff)) ;

  def has_scope_properties($pgrp):
    .pids."\(.pgrps."\($pgrp)"[0])".diff as $diff
    | any(available_in_slice|.[]; has_key($diff)) ;

  def scope_unit: "\(.request.name)-\(.pid).scope" ;

  def user_or_not:
    if .cgroup | test("user.slice"; "") then "--user" else "" end ;

  def move_processes_to_scope($pgrp):
    def count: if .diff | has("cgroup") then "2" else "1" end ;

    def slice_part:
      if .diff | has("cgroup") then ["Slice", "s", "\(slice_unit)"]
      else [] end ;

    .pgrps."\($pgrp)" as $processes
    | .pids."\(.pgrps."\($pgrp)"[0])" |= add_command(
      [
        "busctl", "call", user_or_not, "org.freedesktop.systemd1",
        "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager",
        "StartTransientUnit", "ssa(sv)a(sa(sv))",
        "\(scope_unit)", "fail", count,
        "PIDs", "au", "\($processes | length)"
      ]
      + $processes + slice_part + ["0"]
    )
    | del(.pids."\($processes[])".diff.cgroup) ;

  def unit_property($property):
    .diff."\($property)" as $value
    | "\($property)=\($value)" ;

  def adjust_scope_properties($pgrp):
    .pgrps."\($pgrp)"[0] as $lpid
    # Adjust all properties with a single command
    | .pids."\($lpid)" |= add_command([
      "systemctl", user_or_not, "--runtime",
      "set-property", "\(scope_unit)"
    ])
    | reduce available_in_slice[] as $property (
      . ;
      if .pids."\($lpid)".diff | has($property) then
        .pids."\($lpid)" |= append_to_command([unit_property($property)])
      else . end
    )
    | reduce .pgrps."\($pgrp)"[] as $pid (
      . ; del(.pids."\($pid)".diff."\(available_in_slice[])")
    ) ;

  def adjust_service_properties($pgrp):
    def scope_properties($hash):
      reduce available_in_slice[] as $property (
        [] ;
        if $hash.diff | has($property) then . + [$property]
        else . end
      ) ;

    def service_properties($hash):
      [$hash.entries.slice_properties] + (
        reduce scope_properties($hash)[] as $property (
          [] ; . + [$hash | unit_property($property)]
        )
      ) ;

    def command_prefix($hash):
      if $hash.cgroup | in_own_app_slice then
        ""
      elif $hash.cgroup | in_app_slice then
        # In app.slice, current user must run the command as the slice owner.
        ($hash.cgroup | capture("user@(?<uid>[0-9]+).service") | .uid) as $puid
        | "$SUDO -u \\#\($puid)"
      elif $hash.cgroup | in_system_slice then
        # In system.slice, current user must run the command as root.
        if $uid == 0 then ""
        else "$SUDO" end
      # Detected processes run inside default slices using user or system
      # manager as expected. You should not see this.
      else ("cgroup doesn't match an expected value." | halt_error(1))
      end ;

    # Expect the first pid being the group leader pid
    .pgrps."\($pgrp)"[0] as $lpid
    # Find which property must be set
    | .pids."\($lpid)" as $hash
    | service_properties($hash) as $service_properties
    # Set command prefix when privileges are required.
    | command_prefix($hash) as $prefix
    # Adjust each property with its own command.
    # Set the properties of the leaf service not set the inner nodes' ones. If
    # the cgroup controller for one property is not available, because managed
    # in some slice, another property can still be set when not requiring it.
    | .pids."\($lpid)" |= add_command([
      $prefix, "systemctl", user_or_not, "--runtime",
      "set-property", "\(unit)", $service_properties[]
    ])
    | reduce .pgrps."\($pgrp)"[] as $pid (
      . ; del(.pids."\($pid)".diff."\(available_in_slice[])")
    ) ;

  def manage_process_group($pgrp):
    .pgrps."\($pgrp)"[0] as $lpid
    | if .pids."\($lpid)".diff | length > 0 then
      # Set user processes running outside systemd manager service or scope
      if .pids."\($lpid)".cgroup | in_session_scope then
        # Adjust process properties per process group
        adjust_properties($pgrp)
        # Require moving processes into own scope
        | if require_scope($pgrp) then
          # Require slice
          if .pids."\($lpid)".diff | has("cgroup") then
            start_slice_unit($lpid)
          else . end
          # Start scope unit, moving all processes sharing same process group
          | move_processes_to_scope($pgrp)
          | if has_scope_properties($pgrp) then
            adjust_scope_properties($pgrp)
          else . end
        else . end
      elif .pids."\($lpid)".cgroup | (in_app_slice or in_system_slice) then
        adjust_properties($pgrp)
        # Apply cgroup or scope properties to actual unit
        | if require_scope($pgrp) or has_scope_properties($pgrp) then
          adjust_service_properties($pgrp)
        else . end
      else . end
    # Nothing to do if .diff is empty
    else . end ;

  reduce get_runtime_entries as $entry (
    { "pids": {}, "pgrps": {} } ;
    .pids += { "\($entry.pid)": ($entry + { "commands": [] }) }
    | if .pgrps | has("\($entry.pgrp)") | not then
      .pgrps += { "\($entry.pgrp)": [] }
    else . end
    | .pgrps."\($entry.pgrp)" += [ $entry.pid ]
  )
  # | manage_process_group(.pgrps|keys[])
  | reduce (.pgrps|keys[]) as $pgrp ( . ; manage_process_group($pgrp) )
  # List commands per pgrp
  | [
    .pids[] | select(.commands | (length > 0))
    | [ .pgrp, .request.name, .unit, .pid, .commands ]
  ]
  | if length > 0 then
    reduce .[] as $elem (
      [] ;
      if (length > 0) and .[-1][0] == $elem[0] then
        .[-1][3] += [$elem[3]]
        | .[-1][4] += $elem[4]
      else
        . + [ [ $elem[0], $elem[1], $elem[2], [ $elem[3] ], $elem[4] ] ]
      end
    )
    | .[]
    | "'\(.[1]) (\(.[0])) \(.[2]) (\(.[3]|map(tostring)|join(" ")))'"
    as $msg
    | [ "echo", "──", "\($msg)" ], .[4][]
    | (map(@sh) | join(" "))
  else halt end ;


def manage_runtime:
  [
    "pid",
    "ppid",
    "pgrp",
    "uid",
    "state",
    "slice",
    "unit",
    "comm",
    "cgroup",
    "priority",
    "nice",
    "num_threads",
    "rtprio",
    "policy",
    "oom_score_adj",
    "ioclass",
    "ionice"
  ]
  as $fields
  | map(
    . as $stat
    | reduce range(0; length) as $pos (
      {} ; .  + { "\($fields[$pos])": $stat[$pos] }
    )
  )
  | stream_runtime_adjust ;
  # ) ;

# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
