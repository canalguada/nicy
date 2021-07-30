module { "name": "manage" };

include "./common" ;

def get_runtime_entries:
  rule_names as $names
  | .[]
  # Filter comm without rule
  # TODO: Find better test
  | select([.comm] | _inside($names))
  # Raw pid status entries
  | .
  + { "sched": ["other", "fifo", "rr", "batch", "iso", "idle"][.policy] }
  + ( .comm | fake_values($shell; $nproc; $max_nice) | get_input )
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


def in_current_user_slice: test("user-\($uid).slice"; "") ;


def in_user_slice: test("user-[0-9]+.slice"; "") ;


def in_session_scope: test("user-[0-9]+.slice/session-[0-9]+.scope"; "") ;


def in_app_slice: test("user@[0-9]+.service/app.slice"; "") ;


def in_system_slice: test("system.slice"; "") ;


# Set command prefix when privileges are required.
def sudo_prefix:
  if has("cgroup") then
    if (.cgroup | in_current_user_slice) then
      ""
    # Only root is expected to manage processes outside his slice.
    elif (.cgroup | in_user_slice) then
      # Current user MUST run the command as the slice owner.
      # Environment variables are required for busctl and systemctl commands
      # Alternative to runuser command : sudo -u \\#\(.uid)
      # Reading info from cgroup capture("user-(?<uid>[0-9]+).slice") | .uid
      [
        if $uid == 0 then "" else "$SUDO" end,
        "runuser -u \(.user) --",
        "env",
        "DBUS_SESSION_BUS_ADDRESS=unix:path=/run/user/\(.uid)/bus",
        "XDG_RUNTIME_DIR=/run/user/\(.uid)"
      ]
      | join(" ")
    elif (.cgroup | in_system_slice) then
      # In system.slice, current user must run the command as root.
      if $uid == 0 then ""
      else "$SUDO" end
    # Detected processes run inside default slices, using user or system
    # manager as expected. You should not see this.
    else error("invalid cgroup: '\(.cgroup)'.") end
  else error("missing key: cgroup") end ;


def stream_runtime_adjust:
  def unit: .cgroup | sub(".*/"; "") ;

  def scope_unit: "\(.request.name)-\(.pid).scope" ;

  def user_or_not:
    if .cgroup | test("user.slice"; "") then "--user" else "" end ;

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
    .pids."\($lpid)" |= quiet_command([
      sudo_prefix, "systemctl", user_or_not, "start", slice_unit
    ])
    # Set properties
    | if .pids."\($lpid)" | has_entry("slice_properties") then
      .pids."\($lpid)" |= quiet_command([
        sudo_prefix, "systemctl", user_or_not, "--runtime",
        "set-property", slice_unit, _("slice_properties")
      ])
    else . end ;

  def has_key($entry): . as $i | $entry | has($i) ;

  def require_scope($pgrp):
    .pids."\(.pgrps."\($pgrp)"[0])".diff as $diff
    | any(cgroup_keys|.[]; has_key($diff)) ;

  def has_scope_properties($pgrp):
    .pids."\(.pgrps."\($pgrp)"[0])".diff as $diff
    | any(available_in_slice|.[]; has_key($diff)) ;

  def move_processes_to_scope($pgrp):
    def count: if .diff | has("cgroup") then "2" else "1" end ;

    def slice_part:
      if .diff | has("cgroup") then ["Slice", "s", "\(slice_unit)"]
      else [] end ;

    .pgrps."\($pgrp)" as $processes
    | .pids."\(.pgrps."\($pgrp)"[0])" |= add_command(
      [
        sudo_prefix, "busctl", "call", "--quiet", user_or_not,
        "org.freedesktop.systemd1",
        "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager",
        "StartTransientUnit", "'ssa(sv)a(sa(sv))'",
        "\(scope_unit)", "fail", count,
        "PIDs", "au", ($processes | "\(length)", (map(tostring) |join(" ")))
      ]
      + slice_part + ["0"]
    )
    | del(.pids."\($processes[])".diff.cgroup) ;

  def unit_property($property):
    .diff."\($property)" as $value
    | "\($property)=\($value)" ;

  def adjust_scope_properties($pgrp):
    .pgrps."\($pgrp)"[0] as $lpid
    # Adjust all properties with a single command
    | .pids."\($lpid)" |= add_command([
      sudo_prefix, "systemctl", user_or_not, "--runtime",
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
        [] ; if $hash.diff | has($property) then . + [$property] else . end
      ) ;

    def service_properties($hash):
      [$hash.entries.slice_properties] + (
        reduce scope_properties($hash)[] as $property (
          [] ; . + [$hash | unit_property($property)]
        )
      ) ;

    # Expect the first pid being the group leader pid
    .pgrps."\($pgrp)"[0] as $lpid
    # Find which property must be set
    | .pids."\($lpid)" as $hash
    | service_properties($hash) as $service_properties
    # Adjust each property with its own command.
    # Set the properties of the leaf service. Don't deal with inner nodes. If
    # the cgroup controller for one property is not available, yet managed in
    # some slice, another property can still be set if not requiring it.
    | .pids."\($lpid)" |= add_command([
      sudo_prefix, "systemctl", user_or_not, "--runtime",
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
          if .pids."\($lpid)".diff | has("cgroup") then start_slice_unit($lpid)
          else . end
          # Start scope unit, moving all processes sharing same process group
          | move_processes_to_scope($pgrp)
          | if has_scope_properties($pgrp) then adjust_scope_properties($pgrp)
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
    # | "adjusting comm:\(.[1]) pgrp:\(.[0]) cgroup:\(.[2]) pids:\(.[3]|map(tostring)|join(" "))"
    # as $msg
    # | [ "echo", "\($msg)" ], .[4][]
    # | (map(@sh) | join(" "))
    | { "Pgrp": .[0], "Comm": .[1], "Unit": .[2], "Pids": .[3], "Commands": .[4] }
    # | "\(.)"
  else halt end ;


def manage_runtime:
  # [
  #   "pid",
  #   "ppid",
  #   "pgrp",
  #   "uid",
  #   "user",
  #   "state",
  #   "slice",
  #   "unit",
  #   "comm",
  #   "cgroup",
  #   "priority",
  #   "nice",
  #   "num_threads",
  #   "rtprio",
  #   "policy",
  #   "oom_score_adj",
  #   "ioclass",
  #   "ionice"
  # ]
  # as $fields
  # | map(
  #   . as $stat
  #   | reduce range(0; length) as $pos (
  #     {} ; .  + { "\($fields[$pos])": $stat[$pos] }
  #   )
  # )
  # | stream_runtime_adjust ;
  stream_runtime_adjust ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
