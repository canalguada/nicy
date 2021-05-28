#!/usr/bin/awk -f
#
# manage.awk --- Filter and format cat /proc/*/stat output to be used as jq input
# Ex.
# cat /proc[0-9]*/stat | ./procstat.awk
#
# [
#   "pid",
#   "ppid",
#   "pgrp",
#   "uid",
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

BEGIN { pgrp = PROCINFO["pgrpid"] ; }
$5 !~ pgrp {
# {
  "stat -c %u /proc/" $1 | getline uid
  #UID_SKIP

  getline cgroup < sprintf("/proc/%d/cgroup", $1)
  n = split(cgroup, cgres, "/")
  #SLICE_SKIP

  gsub("[()]", "", $2)
  getline oom_score_adj < sprintf("/proc/%d/oom_score_adj", $1)
  "ionice -p " $1 | getline out
  if (match(out, "(.*): prio(.*)", iores) == 0) {
    iores[2]=0 ;
    if (out == "idle") {
      iores[1]="idle" ;
    } else {
      iores[1]="none" ;
    }
  }

  printf "%s", "[ "
  #       pid, ppid, pgrp, uid, state
  printf "%d, %d, %d, %d, \"%c\", ", \
         $1, $4, $5, uid, $3
  #       slice,  unit,   comm,   cgroup
  printf "\"%s\", \"%s\", \"%s\", \"%s\", ", \
         cgres[2], cgres[n], $2, cgroup
  #       priority, nice, num_threads, rtprio, policy
  printf "%d, %d, %d, %d, %d, ", \
         $18, $19, $20, $40, $41

  #       oom_score_adj, ioclass, ionice
  printf "%d, \"%s\", %d", \
         oom_score_adj, iores[1], iores[2]

  printf "%s", " ]\n"
}

# vim: set ft=awk fdm=marker ts=2 sw=2 tw=79 et:
