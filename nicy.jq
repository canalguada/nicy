module "nicy" ;

# Common functions
# ================

# def to_object($in_array):
#   $in_array
#   | reduce range(0; length; 2) as $i (
#     {}; . += { "\($in_array[$i])": $in_array[$i + 1] }
#   ) ;

def valid_keys:
  [
    "cgroup", "cgroup_properties", "nice",
    "CPUQuota", "IOWeight", "MemoryHigh", "MemoryMax",
    "sched", "rtprio", "ioclass", "ionice", "oom_score_adj",
    "cmdargs", "env"
  ] ;

def cgroup_keys:
  [
    "cgroup", "cgroup_properties", "CPUQuota", "IOWeight",
    "MemoryHigh", "MemoryMax"
  ] ;

def cgroup_paths:
  cgroup_keys | reduce .[] as $key ([]; . + [$key]) ;

def best_matching($key; $object):
  [.[] | if (del(.[$key]) | inside($object)) then . else empty end]
  | sort_by(length)
  | reverse
  | first(.[] | .[$key])? // null ;

# Main filter
# ===========

# Use 'auto' option to get the rule for the 'name' command, if any (default).
# Use 'cgroup-only' to remove everything but the cgroup from the rule.
# Use 'default' or an other defined type.
def get_entries:
  def get_first($key; $value):
    first(.[] | select(.[$key] == ($value|tostring))) ;

  def get_rule($value):
    .rules | get_first("name"; $value) ;

  def get_type_or_die($value):
    .types
    | get_first("type"; $value)? // {}
    | if length == 0 then
      # { "error": "unknown type '\($value)'" }
      "unknown type '\($value)'\n"|halt_error(2)
    else . end ;

  def get_cgroup_or_die($value):
    .cgroups
    | get_first("cgroup"; $value)? // {}
    | if length == 0 then
      # { "error": "unknown cgroup '\($value)'" }
      "unknown cgroup '\($value)'\n"|halt_error(4)
    else . end ;

  def rule_or_default:
    get_rule(.result.request.name)?
    // get_type_or_die("default") ;

  def type_or_die:
    get_type_or_die(.result.request.option) ;

  def load($kind):
    if ($kind == "type") and (.result | has("type")) then
      .result = get_type_or_die(.result.type) + .result
    elif ($kind == "cgroup") and (.result | has("cgroup")) then
      .result = get_cgroup_or_die(.result.cgroup) + .result
    else . end ;

  def cgroup_only:
    .result |= (
      (keys - ["name", "cgroup", "request", "env", "cmdargs"]) as $todel
      | with_entries(if ([.key]|inside($todel)) then empty else . end)
    ) ;

  def move_to_cgroup:
    # Do not use any unknown cgroup
    .result += get_cgroup_or_die(.result.request.use_cgroup) ;

  . + { "result": { "request": $request } }

  # -z, --cgroup-only Unset all other properties.
  | if .result.request.option | test( "^(auto|cgroup-only)$"; "i") then
    .result += rule_or_default
  # -d, --default     Like '--type=default'. Do not search for a rule.
  #                   Apply the fallback values from the 'default' type.
  # -t, --type=TYPE   Use the set of properties of type TYPE.
  else .result += type_or_die end
  | load("type")
  | if .result.request.option == "cgroup-only" then cgroup_only else . end
  | load("cgroup")
  | if .result.request.use_cgroup != null then move_to_cgroup else . end
  # No cgroup defined in configuration files nor .result.requested by user
  | if (.result| has("cgroup") | not) and
    (.result.request.find_matching == true) then
    .result += {
      "cgroup": (
        (.result | delpaths([paths] - cgroup_paths)) as $properties
        | if ($properties | length) == 0 then null
        else .cgroups | best_matching("cgroup"; $properties) end
      )
    }
  # Use cgroup from rule found in configuration file, if any
  else . end
  | del(.result.name, .result.type)
  | if .result.cgroup != null then
    # Extract cgroup properties, if any
    get_cgroup_or_die(.result.cgroup) as $properties
    | .result |= delpaths([$properties | paths] - [["cgroup"]])
    | .result += {
      "cgroup_properties": (
        $properties
        | del(.cgroup)
        | to_entries
        | map("\(.key)=\(.value)")
        | join(" ")
      )
    }
  else del(.result.cgroup) end
  # Interpolation
  | .result ;

def get_commands:
  def set_env:
    if has("env") then
      if .use_scope == false then
        .commands += ["export \(.env | map("\(.)" | @sh) | join(" "))"]
        | del(.env)
      else . end
    else . end ;

  def apply_renice:
    if has("nice") then
      .commands += ["ulimit -S -e \(20 - .nice) &>/dev/null"]
      | if .use_scope == false then
        .commands += ["renice -n \(.nice) &>/dev/null"]
        | del(.nice)
      else . end
    else . end ;

  def apply_choom:
    if has("oom_score_adj") then
      if .use_scope == true then
        .exec_cmd = ["choom -n \(.oom_score_adj) --"] + .exec_cmd
      else
        .commands += ["choom -n \(.oom_score_adj) -p $$ &>/dev/null"]
      end
      | del(.oom_score_adj)
    else . end ;

  def apply_schedtool:
    del(.args[])
    | if has("sched") then
      if .sched == "other" then .args += ["-N"]
      elif .sched == "fifo" then .args += ["-F"]
      elif .sched == "rr" then .args += ["-R"]
      elif .sched == "batch" then .args += ["-B"]
      elif .sched == "idle" then .args += ["-D"]
      else . end
    else . end
    | if has("rtprio") then
      if (has("sched") and ((.sched == "fifo") or (.sched == "rr")))
      or ((has("sched") | not) and (.request.schedtool | test("POLICY (F|R):"; "i"))) then
        .args += ["-p \(.rtprio)"]
      else . end
    else . end
    | del(.sched, .rtprio)
    | if (.args|length) > 0 then
      if .use_scope == true then
        .exec_cmd = ["schedtool"] + .args + ["-e"] + .exec_cmd
      else
        .commands += ["schedtool \(.args|join(" ")) $$ &>/dev/null"]
      end
    else . end ;

  def apply_ionice:
    del(.args[])
    | if has("ioclass") then
      if .ioclass == "none" then .args += ["-c 0"]
      elif .ioclass == "realtime" then .args += ["-c 1"]
      elif .ioclass == "best-effort" then .args += ["-c 2"]
      elif .ioclass == "idle" then .args += ["-c 3"]
      else . end
    else . end
    | if has("ionice") then
      if (has("ioclass") and ((.ioclass == "realtime") or (.ioclass == "best-effort")))
      or ((has("ioclass") | not) and (.request.ionice | test("^(realtime|best-effort):"; "i"))) then
        .args += ["-n \(.ionice)"]
      else . end
    else . end
    | del(.ioclass, .ionice)
    | if (.args|length) > 0 then
      if .use_scope == true then
        .exec_cmd = ["ionice -t"] + .args + .exec_cmd
      else
        .commands += ["ionice -t \(.args|join(" ")) -p $$ &>/dev/null"]
      end
    else . end ;

  def complete_cmd:
    del(.args[])
    | if .use_scope == true then
      .args += ["exec systemd-run ${user_or_system} -G -d --no-ask-password"]
      | if (.request.quiet == true) or (.request.verbose < 2) then
        .args += ["--quiet"]
      else . end
      | .args += ["--unit=\(.request.which_cmd | sub(".*/"; ""; "l"))-$$ --scope"]
      | if has("cgroup") then
        "nicy-\(.cgroup).slice" as $slice
        | .commands += [
          "systemctl ${user_or_system} start \($slice) &>/dev/null"
        ]
        | if has("cgroup_properties") then
          .commands += [
            "systemctl ${user_or_system} --runtime"
            + " set-property \($slice) \(.cgroup_properties) &>/dev/null"
          ]
          | del(.cgroup_properties)
        else . end
        | .args += ["--slice=\($slice)"]
        | del(.cgroup)
      else . end
      | if has("CPUQuota") then
        .args += [
          "-p",
          # The nicy quota value is a percentage of total CPU time for ALL cores
          "CPUQuota=\((.CPUQuota | rtrimstr("%") | tonumber) * .request.nproc)%"
        ]
        | del(.CPUQuota)
      else . end
      | if has("IOWeight") then
        .args += [ "-p", "IOWeight=\(.IOWeight)" ]
        | del(.IOWeight)
      else . end
      | if has("MemoryHigh") then
        .args += [ "-p", "MemoryHigh=\(.MemoryHigh)" ]
        | del(.MemoryHigh)
      else . end
      | if has("MemoryMax") then
        .args += [ "-p", "MemoryMax=\(.MemoryMax)" ]
        | del(.MemoryMax)
      else . end
      | if has("nice") then
        .args += [ "--nice=\(.nice)" ]
        | del(.nice)
      else . end
      | if has("env") then
        .args += (.env | map("-E", "\(.)"))
        | del(.env)
      else . end
    else .args += ["exec"] end
    | if has("cmdargs") then
      .exec_cmd += .cmdargs
      | del(.cmdargs)
    else . end
    | .commands += [(.args + .exec_cmd) | join(" ")] ;

  . + { "commands": [], "exec_cmd": ["\(.request.which_cmd)"], "args": [] }
  | if (.request.use_scope == true) or has("cgroup") then
    . + { "use_scope": true }
  else . + { "use_scope": false } end
  | set_env
  | if .use_scope == true then
    # No authentication for privileged operations.
    .commands += ["[ $UID -ne 0 ] && user_or_system=--user"]
  else . end
  | apply_renice
  | apply_choom
  | apply_schedtool
  | apply_ionice
  | complete_cmd ;

def main:
  get_entries
  | get_commands
  | if has("error") then
     "error", .error
   else "commands", (.commands | .[] | @sh)
  end ;

# Parsing functions
# =================

def is_percentage:
  tostring | test("^[1-9][0-9]?%?$"; "") ;

def is_bytes:
  tostring | test("^[1-9][0-9]+(K|M|G|T)?$"; "i") ;

def in_range($from; $upto):
  tonumber | . >= $from and . <= $upto ;

def object_to_array:
  to_entries | map("\(.key)=\(.value)"|@sh) ;

def object_to_string:
  object_to_array | join(" ") ;

def array_to_string:
  map("\(.)"|@sh) | join(" ");

def parse_entry:
  def check_memory_value:
    if .value == "infinity" then .
    elif .value | is_percentage then .value |= "\(rtrimstr("%"))%"
    elif .value | is_bytes then .value |= ascii_upcase
    else empty end ;

  .key |= ascii_downcase
  |	if .key | test("^(name|type|cgroup)$"; "") then .
  else
    try
      # Adjust percentage with nproc
      if (.key == "cpuquota") and (.value | is_percentage) then
        .key |= "CPUQuota"
        | .value |= "\((rtrimstr("%")|tonumber) * ($nproc|tonumber))%"
      elif (.key == "ioweight") and (.value | in_range(1; 10000)) then
        .key |= "IOWeight"
      elif .key == "memoryhigh" then
        .key |= "MemoryHigh" | check_memory_value
      elif .key == "memorymax" then
        .key |= "MemoryMax" | check_memory_value
      elif (.key == "sched" and
        (.value | test("^(other|fifo|rr|batch|idle)$"; ""))) or
        ((.key == "rtprio") and (.value | in_range(1; 99))) or
        ((.key == "nice") and (.value | in_range(-20; 19))) or
        ((.key == "ioclass") and
         (.value | tostring | test("^(none|realtime|best-effort|idle)$"; ""))) or
        ((.key == "ionice") and (.value | in_range(0; 7))) or
        ((.key == "oom_score_adj") and (.value | in_range(-1000; 1000))) then
        .
      elif .key == "cmdargs" then
        if (.value | type) == "array" then .
        elif (.value | type) == "string" then .value |= [.value]
        else empty end
      elif .key == "env" then
        if (.value | type) == "object" then .value |= object_to_array
        elif (.value | type) == "array" then .
        elif (.value | type) == "string" then .value |= [.value]
        else empty end
      else empty end
    catch empty
  end ;

def entries_downcase:
  with_entries(.key |= ascii_downcase) ;

def make_cache($kind):
  unique_by(.[$kind]) | .[]
  | if type == "object" then with_entries(parse_entry)
  else .  end ;

# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
