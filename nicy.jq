module "nicy" ;

# Common functions
# ================

# def to_object($in_array):
#   $in_array
#   | reduce range(0; length; 2) as $i (
#     {}; . += { "\($in_array[$i])": $in_array[$i + 1] }
#   ) ;

def cgroups:
  $cachedb.cgroups;

def types:
  $cachedb.types;

def rules:
  $cachedb.rules;

def get_rule($value):
  first(rules[] | select(.name == $value)) ;

def get_type_or_die($value):
  first(types[] | select(.type == $value))? // {}
  | if length == 0 then
    # { "error": "unknown type '\($value)'" }
    "unknown type '\($value)'\n"|halt_error(2)
  else . end ;

def get_cgroup_or_die($value):
  first(cgroups[] | select(.cgroup == $value))? // {}
  | if length == 0 then
    # { "error": "unknown cgroup '\($value)'" }
    "unknown cgroup '\($value)'\n"|halt_error(4)
  else . end ;

def available_in_slice:
  ["CPUQuota", "IOWeight", "MemoryHigh", "MemoryMax"] ;

def cgroup_keys:
  ["cgroup"] + available_in_slice ;

def type_keys:
  ["type"]
  + ["nice", "sched", "rtprio", "ioclass", "ionice", "oom_score_adj"]
  + cgroup_keys ;

def rule_keys:
  ["name"] + type_keys + ["cmdargs", "env"] ;

def cgroup_paths:
  reduce cgroup_keys[] as $key ([]; . + [$key]) ;

def is_percentage:
  tostring | test("^[1-9][0-9]?%?$"; "") ;

def is_bytes:
  tostring | test("^[1-9][0-9]+(K|M|G|T)?$"; "i") ;

def in_range($from; $upto):
  tonumber | . >= $from and . <= $upto ;

def in_array($content):
  [.] | inside($content) ;

def object_to_array:
  to_entries | map("\(.key)=\(.value)"|@sh) ;

def object_to_string:
  object_to_array | join(" ") ;

def array_to_string:
  map("\(.)"|@sh) | join(" ");

def best_matching($key; $object):
  # [.[] | if (del(.[$key]) | inside($object)) then . else empty end]
  [.[] | select(del(.[$key]) | inside($object))]
  | sort_by(length)
  | reverse
  | first(.[] | .[$key])? // null ;

def has_entry($p):
  .entries | has($p) ;

def _($p):
  .entries."\($p)" ;

def del_entry(f):
  if f|type == "string" then
    del(.entries."\(f)")
  else reduce f[] as $p (.; del(.entries."\($p)")) end ;

# Nicy quota value is a percentage of total CPU time for ALL cores
def cpuquota_adjust($cores):
  "\((rtrimstr("%") | tonumber) * $cores)%" ;

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

# Use 'auto' option to get the rule for the 'name' command, if any (default).
# Use 'cgroup-only' to remove everything but the cgroup from the rule.
# Use 'default' or an other defined type.
def get_entries:
  def rule_or_default:
    get_rule(.request.name)?
    // get_type_or_die("default") ;

  def type_or_die:
    get_type_or_die(.request.option) ;

  def keep_cgroup_only:
    del_entry((.entries|keys) - ["name", "cgroup", "env", "cmdargs"]) ;

  def move_to_cgroup:
    # Do not use any unknown cgroup
    .entries += get_cgroup_or_die(.request.use_cgroup) ;

  def current_cgroup_properties:
    .entries | delpaths([paths] - cgroup_paths) ;

  def format_slice_properties:
    (get_cgroup_or_die(_("cgroup")) | del(.cgroup, .origin)) as $properties
    | .request.nproc as $cores
    # | .entries |= delpaths([$properties | paths])
    | del_entry($properties | keys)
    | .entries += {
      "slice_properties": (
        $properties
        | if has("CPUQuota") then
          .CPUQuota |= cpuquota_adjust($cores)
        else .  end
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
    }
  ;

  def rule_or_type:
    # if .request.option | test( "^(auto|cgroup-only)$"; "i")
    if .request.option | in_array(["auto", "cgroup-only"])
    then rule_or_default
    else type_or_die
    end ;

  def load_from($r):
    (if $r | has("type") then get_type_or_die($r.type) else null end)
    as $typedef
    | (if $r | has("cgroup") then get_cgroup_or_die($r.cgroup) else null end)
    as $cgroupdef
    | .entries += $typedef + $cgroupdef + ($r | del(.type, .cgroup)) ;

  load_from(rule_or_type)

  | if .request.option == "cgroup-only" then keep_cgroup_only else . end
  | if .request.use_cgroup != null then move_to_cgroup else . end
  # No cgroup defined in configuration files nor _("requested") by user
  | if (has_entry("cgroup") | not) and (.request.find_matching == true) then
      move_to_matching_cgroup
  # Use cgroup from rule found in configuration file, if any
  else . end
  | del_entry(["name", "type"])
  | if _("cgroup") != null then
    # Extract and format slice properties, if any
    format_slice_properties
  else del_entry("cgroup") end ;

def get_commands:
  def add_command($cmdargs):
    .commands += [$cmdargs] ;

  def process_env:
    if has_entry("env") then
      if .use_scope then .
      else
        add_command(["export"] + _("env"))
        | del_entry("env")
      end
    else . end ;

  def process_nice:
    if has_entry("nice") then
      add_command(["ulimit", "-S", "-e", "\(20 - _("nice"))", "&>/dev/null"])
      | if .use_scope then .
      else
        add_command(["renice", "-n", "\(_("nice"))", "&>/dev/null"])
        | del_entry("nice")
      end
    else . end ;

  def process_oom_score_adj:
    if has_entry("oom_score_adj") then
      if .use_scope then
        .exec_cmd += ["choom", "-n", "\(_("oom_score_adj"))", "--"]
      else
        add_command([
          "choom", "-n", "\(_("oom_score_adj"))", "-p", "$$", "&>/dev/null"
        ])
      end
      | del_entry("oom_score_adj")
    else . end ;

  def process_sched_rtprio:
    def schedtool_args:
      def get_sched($key):
        [
          {
            "other": "-N",
            "fifo": "-F",
            "rr": "-R",
            "batch": "-B",
            "idle": "-D"
          }[$key]
        ] ;

      def get_rtprio($flag):
        .entries
        | if (has("sched") and ((.sched == "fifo") or (.sched == "rr")))
        or ((has("sched") | not) and $flag) then
          ["-p", "\(.rtprio)"]
        else null end ;

      . + { "args": [] }
      | if (has_entry("sched")) then
        try
          (.args += [get_sched(_("sched"))])
        catch ("bad sched value '\(_("sched"))'" | halt_error(8))
      else . end
      | if (has_entry("rtprio")) then
        .args += get_rtprio(.request.schedtool | test("POLICY (F|R):"; "i"))
      else . end
      | .args ;

    schedtool_args as $args
    | del_entry(["sched", "rtprio"])
    | if ($args|length) > 0 then
      if .use_scope then
        .exec_cmd += ["schedtool"] + $args + ["-e"]
      else
        add_command(["schedtool"] + $args + ["$$", "&>/dev/null"])
      end
    else . end ;

  def process_ioclass_ionice:
    def ionice_args:
      def get_ioclass($key):
        [
          {
            "none": "0",
            "realtime": "1",
            "best-effort": "2",
            "idle": "3"
          }[$key]
        ] ;

      def get_ionice($flag):
        .entries
        | if (has("ioclass") and
          ([.ioclass] | inside(["realtime", "best-effort"])))
        or ((has("ioclass") | not) and $flag) then
          ["-n", "\(.ionice)"]
        else null end ;

      . + { "args": [] }
      | if (has_entry("ioclass")) then
        try (.args += ["-c", get_ioclass(_("ioclass"))])
        catch ("bad ioclass value '\(_("ioclass"))'"|halt_error(8))
      else . end
      | if (has_entry("ionice")) then
        # .args += get_ionice(.request.ionice | test("^(realtime|best-effort):"; "i"))
        .args += get_ionice(.request.ionice | in_array(["realtime", "best-effort"]))
      else . end
      | .args ;

    ionice_args as $args
    | del_entry(["ioclass", "ionice"])
    | if ($args|length) > 0 then
      if .use_scope then
        .exec_cmd += ["ionice", "-t"] + $args
      else
        add_command(["ionice", "-t"] + $args + ["-p", "$$", "&>/dev/null"])
      end
    else . end ;

  def process_exec:
    def build($property):
      if has_entry($property) then
        _($property) as $value
        | del_entry($property)
        | [ "-p", "\($property)=\($value)" ]
      else null end ;

    def prepare_scope_args:
      .exec_args += [
        "systemd-run", "${user_or_system}", "-G", "-d", "--no-ask-password"
      ]
      | if .request.quiet or (.request.verbose < 2) then
        .exec_args += ["--quiet"]
      else . end
      | (.request.which_cmd | sub(".*/"; ""; "l")) as $name
      | .exec_args += ["--unit=\($name)-$$", "--scope"] ;

    def slice_unit:
      "nicy-\(_("cgroup")).slice" ;

    def cmd_start_slice_unit:
      add_command([
        "systemctl", "${user_or_system}", "start", slice_unit, "&>/dev/null"
      ]) ;

    def cmd_set_slice_properties:
      add_command([
        "systemctl", "${user_or_system}", "--runtime",
        "set-property", slice_unit, _("slice_properties"), "&>/dev/null"
      ])
      | del_entry("slice_properties") ;

    . + { "exec_args": [] }
    # Use systemd scope unit
    | if .use_scope then
      prepare_scope_args
      # Run inside some cgroup
      | if has_entry("cgroup") then
        slice_unit as $slice
        # Start slice
        | cmd_start_slice_unit
        # Set properties
        | if has_entry("slice_properties") then
          cmd_set_slice_properties
        else . end
        | .exec_args += ["--slice=\($slice)"]
        | del_entry("cgroup")
      else . end
      # Adjust scope properties, thus outside cgroup/slice
      | if has_entry("CPUQuota") then
        _("CPUQuota") |= cpuquota_adjust(.request.nproc)
      else . end
      | reduce available_in_slice[]
      as $property (. ; .exec_args += build($property))
      | if has_entry("nice") then
        .exec_args += [ "--nice=\(_("nice"))" ]
        | del_entry("nice")
      else . end
      | if has_entry("env") then
        .exec_args += (_("env") | map("-E", "\(.)"))
        | del_entry("env")
      else . end
    else . end
    | .exec_cmd += [.request.which_cmd]
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

  | if .request.use_scope or (has_entry("cgroup")) then
    .use_scope = true
  else . end
  | process_env
  | if .use_scope then
    # No authentication here for privileged operations.
    add_command(["[ $UID -ne 0 ] && user_or_system=--user"])
  else . end
  | process_nice
  | process_oom_score_adj
  | process_sched_rtprio
  | process_ioclass_ionice
  | process_exec ;

def main:
  # Expects request as input and cachedb as named json arg
  { "request": ., "entries": {} }
  | get_entries
  | get_commands
  | if has("error") then
     "error", .error
   else "commands", (.commands[] | (map(@sh) | join(" ")))
  end ;

# Parsing functions
# =================

# Validate entry, ignore unknown keys or raise error on bad value
def parse_entry($kind):
  def check_memory_value:
    if .value == "infinity" then .
    elif .value | is_percentage then .value |= "\(rtrimstr("%"))%"
    elif .value | is_bytes then .value |= ascii_upcase
    else error("bad value : \(.value)") end ;

  def parse_cpuquota:
    if .value | is_percentage then
      .key |= "CPUQuota"
    else error("bad value : \(.value)") end ;

  def parse_ioweight:
    if .value | in_range(1; 10000) then
      .key |= "IOWeight"
    else error("bad value : \(.value)") end ;

  def parse_memoryhigh:
    .key |= "MemoryHigh"
    | check_memory_value ;

  def parse_memorymax:
    .key |= "MemoryMax"
    | check_memory_value ;

  def parse_nice:
    if .value | in_range(-20; 19) then .
    else error("bad value : \(.value)") end ;

  def parse_sched:
    if .value | in_array(["other", "fifo", "rr", "batch", "idle"]) then .
    else error("bad value : \(.value)") end ;

  def parse_rtprio:
    if .value | in_range(1; 99) then .
    else error("bad value : \(.value)") end ;

  def parse_ioclass:
    if .value | in_array(["none", "realtime", "best-effort", "idle"]) then .
    else error("bad value : \(.value)") end ;

  def parse_ionice:
    if .value | in_range(0; 7) then .
    else error("bad value : \(.value)") end ;

  def parse_oom_score_adj:
    if .value | in_range(-1000; 1000) then .
    else error("bad value : \(.value)") end ;

  def parse_cmdargs:
    if (.value | type) == "array" then .
    elif (.value | type) == "string" then .value |= [.value]
    else error("bad value : \(.value)") end ;

  def parse_env:
    if (.value | type) == "object" then .value |= object_to_array
    elif (.value | type) == "array" then .
    elif (.value | type) == "string" then .value |= [.value]
    else error("bad value : \(.value)") end ;

  label $out
  | (
    if $kind == "rule" then
      rule_keys + ["origin", "type", "cgroup"]
    elif $kind == "type" then
      type_keys + ["origin", "cgroup"]
    elif $kind == "cgroup" then
      cgroup_keys + ["origin"]
    else empty end
  ) as $keys
  | .key |= ascii_downcase
  | if .key | in_array($keys | map(ascii_downcase)) | not then
    break $out
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
  elif [.key] | inside(rule_keys - type_keys) then
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
  | if type == "object" then
    with_entries(parse_entry($kind))
  # Raise error
  else ("can't parse '\(type)' input building cache" | halt_error(1)) end ;

# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
