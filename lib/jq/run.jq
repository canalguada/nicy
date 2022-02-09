module { "name": "run" };

include "./common" ;


# Expects request values as object
# {
#   "name": "name",
#   "cmd": "/path/to/command",
#   "preset": "auto",
#   "cgroup": "",
#   "probe_cgroup": false,
#   "managed": false,
#   "quiet": true,
#   "verbosity": 0,
#   "shell": "/path/to/shell",
#   "nproc": 1,
#   "max_nice": 20,
#   "cpusched": "0:other:0",
#   "iosched": "0:none:0"
# }
#
def run:
  get_input
  | get_entries
  | get_commands
  | if has("error") then "error", .error
  else "commands", .commands[] end ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
