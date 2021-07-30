module { "name": "run" };

include "./common" ;


# Expects request values as string array
def run:
  map(jsonify)
  | get_input
  | get_entries
  | get_commands
  | if has("error") then "error", .error
  # else "commands", (.commands[] | (map(@sh) | join(" "))) end ;
  else "commands", .commands[] end ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
