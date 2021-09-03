module { "name": "manage" };

include "./common" ;

# Testing .cgroup
# ===============

def in_current_user_slice: test("user-\($uid).slice"; "") ;

def in_user_slice: test("user-[0-9]+.slice"; "") ;

def in_session_scope: test("user-[0-9]+.slice/session-[0-9]+.scope"; "") ;

def in_app_slice: test("user@[0-9]+.service/app.slice"; "") ;

def in_system_slice: test("system.slice"; "") ;

def in_nicy_cgroup($c): test("nicy.slice/nicy-\($c).slice"; "") ;


# Building `manage_process_group` input
# ======================================
# [
#   {
#     "pgrp": 7081,
#     "pids": [
#       7097,
#       ...
#     ],
#     "procs": [
#       {
#         # Raw `manage_runtime` input
#         "pid": 7097,
#         "ppid": 1,
#         "pgrp": 7081,
#         "uid": 1001,
#         "user": "canalguada",
#         "state": "S",
#         "slice": "user.slice",
#         "unit": "nvim-qt-7097.scope",
#         "comm": "nvim-qt",
#         "cgroup":
#         "0::/user.slice/user-1001.slice/user@1001.service/nicy.slice/nicy-cpu33.slice/nvim-qt-7097.scope",
#         "priority": 17,
#         "num_threads": 8,
#         "runtime": {
#           "nice": -3,
#           "sched": "other",
#           "rtprio": 0,
#           "ioclass": "none",
#           "ionice": 0,
#           "oom_score_adj": 0,
#         },
#         # Added in `get_runtime_entries`
#         "request": {
#           "name": "nvim-qt",
#           "cmd": "%nvim-qt%",
#           "preset": "auto",
#           "cgroup": "",
#           "probe_cgroup": false,
#           "managed": false,
#           "quiet": true,
#           "verbosity": 0,
#           "shell": "/bin/sh",
#           "nproc": 1,
#           "max_nice": 20,
#           "cpusched": "0:other:0",
#           "iosched": "0:none:0"
#         },
#         "entries": {
#           "cred": [],
#           ...
#         },
#         "diff": {},
#         "commands": []
#       },
#       ...
#     ]
#   },
#   ...
# ]

# Raw pid status entries sorted by pid and filtering comm without rule
def get_runtime_input:
  [ .[] | select(.comm | within(rule_names)) ] ;

def get_runtime_entries:
  def add_request_and_entries:
    . + (
      .comm
      | fake_values($shell; $nproc; $max_nice)
      | get_input 
    ) ;
  def review_diff:
    (.entries | keys | . - ( . - type_keys )) as $entry_keys
    | reduce ($entry_keys | .[]) as $k (
      . ;
      if .runtime.[$k] != .entries.[$k] then
        .diff += { $k: .entries.[$k] }
      else . end
    ) ;
  # Value for `cgroup` never match 
  def check_cgroup:
    if .diff | has("cgroup") then
      .diff.cgroup as $cgroup
      # Running yet inside its nicy cgroup
      | if (.cgroup | in_nicy_cgroup($cgroup)) then
        del(.diff.cgroup)
      else . end
    else . end ;

  .[]
  | add_request_and_entries
  | get_entries
  | review_diff
  | check_cgroup ;


def prepare_stream:
  reduce get_runtime_entries as $entry (
    [] ; if length == 0 or ($entry.pgrp != .[-1].pgrp) then
      . + [{ "pgrp": $entry.pgrp, "pids": [$entry.pid], "procs": [$entry] }]
    # With same process group
    else
      .[-1].pids += [$entry.pid]
      | .[-1].procs += [$entry]
    end
  ) ;

# Managing process entry
# ======================

def user_or_not:
  if .cgroup | test("user.slice"; "") then "--user" else "--system" end ;

def scope_unit: "\(.request.name)-\(.pid).scope" ;

def unit_property($property):
  .diff."\($property)" as $value
  | "\($property)=\($value)" ;

def unit: .cgroup | sub(".*/"; "") ;

# Set command prefix when privileges are required.
def sudo_prefix:
  if has("cgroup") then
    if (.cgroup | in_current_user_slice) then ""
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

# Managing process group entry
# ============================

def adjust_properties:
  if .procs[0].diff | has("nice") then
    # renice can set niceness per process group
    .procs[0] |= process_nice(null; .pgrp)
  else . end
  | if .procs[0].diff | (has("ioclass") or has("ionice")) then
    # ionice can set io scheduling class and priority per process group
      .procs[0] |= process_ioclass_ionice(null; .pgrp)
    else . end
  # adjustments per process
  | reduce range(0; .procs | length) as $i (
    . ;
    if .procs[$i].diff | has("oom_score_adj") then
      .procs[$i] |= process_oom_score_adj(.pid)
    else . end
    | if .procs[$i].diff | (has("sched") or has("rtprio")) then
      .procs[$i] |= process_sched_rtprio(.pid)
    else . end
    # remove diff entries
    | del(.procs[$i].diff."\((type_keys - cgroup_keys)[])")
  ) ;

def require_sd_unit:
  .procs[0].diff as $diff
  | any(cgroup_keys[]; . | in($diff)) ;

def start_slice_unit:
  .procs[0] |= quiet_command([
    sudo_prefix, "systemctl", user_or_not, "start", slice_unit
  ])
  # Set properties
  | if .procs[0] | has_entry("slice_properties") then
    .procs[0] |= quiet_command([
      sudo_prefix, "systemctl", user_or_not, "--runtime",
      "set-property", slice_unit, _("slice_properties")
    ])
  else . end ;

def move_processes_to_scope_unit:
  def count_args: if .diff | has("cgroup") then "2" else "1" end ;
  def optional_slice_part:
    if .diff | has("cgroup") then ["Slice", "s", "\(slice_unit)"]
    else [] end ;

  .pids as $pids
  | .procs[0] |= add_command(
    [
      sudo_prefix, "busctl", "call", "--quiet", user_or_not,
      "org.freedesktop.systemd1",
      "/org/freedesktop/systemd1", "org.freedesktop.systemd1.Manager",
      "StartTransientUnit", "ssa(sv)a(sa(sv))", "\(scope_unit)",
      "fail", count_args,
      "PIDs", "au", ($pids | "\(length)", (map(tostring) | .[]))
    ]
    + optional_slice_part
    + ["0"]
  )
  | reduce range(0; .procs | length) as $i (
    . ; del(.procs[$i].diff.cgroup)
  ) ;

def has_sd_unit_properties:
  .procs[0].diff as $diff
  | any(available_in_slice[]; . | in($diff)) ;

def remove_available_in_slice:
  reduce range(0; .procs | length) as $i (
    . ; del(.procs[$i].diff."\(available_in_slice[])")
  ) ;

# Adjust all properties within a single command
def adjust_scope_properties:
  .procs[0] |= add_command([
    sudo_prefix, "systemctl", user_or_not, "--runtime",
    "set-property", "\(scope_unit)"
  ])
  | reduce available_in_slice[] as $property (
    . ;
    if .procs[0].diff | has($property) then
      .procs[0] |= append_to_command([unit_property($property)])
    else . end
  )
  | remove_available_in_slice ;

# Adjust each property with its own command.
# Set the properties of the leaf service. Don't deal with inner nodes. If
# the cgroup controller for one property is not available, yet managed in
# some slice, another property can still be set if not requiring it.
def adjust_service_properties:
  def service_properties:
    . as $proc
    # Find which property must be set
    | reduce available_in_slice[] as $property (
        [$proc.entries.slice_properties] ;
        if $proc.diff | has($property) then
          . + [$proc | unit_property($property)]
        else . end
    ) ;

  .procs[0] |= add_command([
    sudo_prefix, "systemctl", user_or_not, "--runtime",
    "set-property", "\(unit)", service_properties[]
  ])
  | remove_available_in_slice ;

def manage_process_group:
  if .procs[0].diff | length > 0 then
    # Expect the first process being the process group leader
    # Set user processes running in session scope
    if .procs[0].cgroup | in_session_scope then
      # Adjust process properties
      adjust_properties
      # Require moving processes into own scope
      | if require_sd_unit then
        # Also require slice
        if .procs[0].diff | has("cgroup") then
          start_slice_unit
        else . end
        # Start scope unit, moving all processes sharing same process group
        | move_processes_to_scope_unit
        | if has_sd_unit_properties then
          adjust_scope_properties
        else . end
      else . end
    # Set user processes controlled by systemd manager
    elif .procs[0].cgroup | (in_app_slice or in_system_slice) then 
      adjust_properties
      # Apply cgroup or scope properties to current unit
      | if require_sd_unit or has_sd_unit_properties then
        adjust_service_properties
      else . end
    else . end
  # Nothing to do if .diff is empty
  else . end ;

def list_commands_by_process_group:
  [
    .procs[]
    | select(.commands | (length > 0))
    | [.pgrp, .request.name, .unit, [.pid], .commands]
  ]
  | if length > 0 then
    # Group by process group
    reduce .[] as $elem (
      [] ;
      # Same process group, use previous group
      if (length > 0) and .[-1][0] == $elem[0] then
        .[-1][3] += $elem[3]
        | .[-1][4] += $elem[4]
      # Add new group:      pgrp, comm, unit, pid,  commands
      else . + ($elem | [ [ .[0], .[1], .[2], .[3], .[4] ] ]) end
    )
    | .[]
    | {
      "Pgrp": .[0], "Comm": .[1], "Unit": .[2], "Pids": .[3], "Commands": .[4]
    }
  else empty end ;

def manage_runtime:
  get_runtime_input
  | prepare_stream
  | .[]
  | manage_process_group
  | list_commands_by_process_group ;

# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
