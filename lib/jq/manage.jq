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

def in_any_nicy_cgroup: test("nicy.slice/nicy-.*.slice"; "") ;


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
  # `cgroup` is always added to diff and need a more specific check
  def check_cgroup:
    if .diff | has("cgroup") then
      .diff.cgroup as $cgroup
      # Processes are quite easily movable when running inside session scope
      # and some managed nicy slice
      # Running yet inside its proper cgroup
      | if (.cgroup | in_nicy_cgroup($cgroup)) then
        del(.diff.cgroup)
      # Keep cgroup in diff
      elif (.cgroup | in_any_nicy_cgroup) or (.cgroup | in_session_scope) then
        .
      # Remove cgroup from diff since processes run probably in some service
      else
        del(.diff.cgroup)
      end
    else . end ;

  .[]
  | add_request_and_entries
  | get_entries
  | review_diff
  | check_cgroup ;


def prepare_stream:
  sort_by(.pgrp)
  | reduce get_runtime_entries as $entry (
    [] ; if length == 0 or ($entry.pgrp != .[-1].pgrp) then
      . + [{ "pgrp": $entry.pgrp, "pids": [$entry.pid], "procs": [$entry] }]
    # With same process group
    else
      .[-1].pids += [$entry.pid]
      | .[-1].procs += [$entry]
    end
  ) ;

def get_process_group_job:
  get_runtime_input
  | prepare_stream
  | .[] | select(.procs[0].diff | length > 0 )
  | {
    "pgrp": .pgrp,
    "pids": .pids,
    "diff": .procs[0].diff,
    "commands": [],
    "procs": .procs
  } ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
