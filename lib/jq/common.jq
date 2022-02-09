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
# def cpuquota_adjust($cores): "\((rtrimstr("%") | tonumber) * $cores)%" ;

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
  # | .request.nproc as $cores
  | del_entry($properties | keys)
  | .entries += {
    "slice_properties": (
      $properties
      # TODO: check where adjustment must be applied
      # | if has("CPUQuota") then .CPUQuota |= cpuquota_adjust($cores)
      # else . end
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


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
