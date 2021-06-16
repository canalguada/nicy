module { "name": "rebuild" };

include "./common" ;


# Validate entry, ignore unknown keys or raise error on bad value
def parse_entry:
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
  # TODO: Change "type" key (used for backward compatibility with Ananicy) into
  # "class" or "family" key in cache
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


def make_cache:
  (if $kind == "rule" then "name" else $kind end) as $key
  # Keep all objects in order to filter per origin later, if required
  # When parsing, only remove optional entries
  | unique_by(.origin, ."\($key)")
  | .[]
  | if type == "object" then with_entries(parse_entry)
  # Raise error
  # else ("cannot parse '\(type)' input building cache" | halt_error(1)) end ;
  else error("cannot parse '\(type)' input building cache") end ;




# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
