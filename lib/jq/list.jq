module { "name": "list" } ;

include "./common" ;


def dump($kind; $dirs):
  [ $cachedb."\($kind)s"[]
    |select(.origin|in_array($dirs))
  ] | (if $kind == "rule" then "name" else "\($kind)" end) as $key
  | unique_by(."\($key)")
  | map([."\($key)", "\(del(.origin))"]) as $rows
  | ["\($key)", "content"] as $cols
  | $cols, $rows[]
  | @tsv ;


def list:
  . as $dirs
  | dump($kind; $dirs) ;


# vim: set ft=jq fdm=indent ai ts=2 sw=2 tw=79 et:
