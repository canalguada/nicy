module { "name": "install" };

include "./common" ;


def stream_scripts_from_rules:
  . as $comms
  | (rule_names - $comms)[]
  | fake_values($shell; $nproc; $max_nice)
  | get_input
  | get_entries
  | get_commands
  | if has("error") then "error", .error
  else
    "begin \(.request.name)",
    "'#!\(.request.shell)'",
    (.commands[] | (map(@sh) | join(" "))),
    "end \(.request.name)"
  end ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
