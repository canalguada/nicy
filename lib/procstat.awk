# procstat.awk --- Filter and format cat /proc/*/stat output to be used as jq input
# Ex.
# cat /proc[0-9]*/stat | ./procstat.awk
#
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

BEGIN { pgrp = PROCINFO["pgrpid"] ; }
$5 !~ pgrp {
# {
  "stat -c '%u %U' /proc/" $1 | getline out
  if (match(out, "(.*) (.*)", uid) == 0) {
    uid[1]=0 ;
    uid[2]="root" ;
  }
  #UID_SKIP

  getline cgpath < sprintf("/proc/%d/cgroup", $1)
  gsub(/\\/, "\\\\", cgpath)
  n = split(cgpath, cgroup, "/")
  #SLICE_SKIP

  gsub("[()]", "", $2)
  getline score_adj < sprintf("/proc/%d/oom_score_adj", $1)
  "ionice -p " $1 | getline out
  if (match(out, "(.*): prio(.*)", io) == 0) {
    io[2]=0 ;
    if (out == "idle") {
      io[1]="idle" ;
    } else {
      io[1]="none" ;
    }
  }

  printf "%s", "[ "
  #       pid, ppid, pgrp, uid, user, state
  printf "%d, %d, %d, %d, \"%s\", \"%c\", ", \
         $1, $4, $5, uid[1], uid[2], $3
  #       slice,  unit,   comm,   cgroup
  printf "\"%s\", \"%s\", \"%s\", \"%s\", ", \
         cgroup[2], cgroup[n], $2, cgpath
  #       priority, nice, num_threads, rtprio, policy
  printf "%d, %d, %d, %d, %d, ", \
         $18, $19, $20, $40, $41
  #       oom_score_adj, ioclass, ionice
  printf "%d, \"%s\", %d", \
         score_adj, io[1], io[2]
  printf "%s", " ]\n"
}

# vim: set ft=awk fdm=marker ts=2 sw=2 tw=79 et:
