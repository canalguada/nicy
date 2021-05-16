module "usage" ;

def dump_options:
  map("  \(.)") | .[] ;

def dump_details:
  map("\(.)", "") | .[] ;

def main:
  "\(.program) version \(.version)",
  "\n\(.description)",
  "\nUsage:",
  "  \(.program) \(.usage.run)",
  "    \(.text.run)",
  "  \(.program) \(.usage.show)",
  "    \(.text.show)",
  "  \(.program) \(.usage.list)",
  "    \(.text.list)",
  "  \(.program) \(.usage.install)",
  "    \(.text.install)",
  "  \(.program) \(.usage.rebuild)",
  "    \(.text.rebuild)",
  "\nCommon options:",
  (.options.help | dump_options),
  "\nRun and show options:",
  (.options.show | dump_options),
  "\nRun only options:",
  (.options.run | dump_options),
  "\nList options:",
  (.options.list | dump_options),
  "",
  (.details.run | dump_details),
  (.details.list | dump_details),
  (.details.install | dump_details) ;

def run:
  "\(.program) version \(.version)",
  "\nUsage:",
  "  \(.program) \(.usage.run)",
  "    \(.text.run)",
  "\nRun options:",
  (.options.show | dump_options),
  "",
  (.options.run | dump_options),
  "",
  (.options.help | dump_options),
  "",
  (.details.run | dump_details) ;

def show:
  "\(.program) version \(.version)",
  "\nUsage:",
  "  \(.program) \(.usage.show)",
  "    \(.text.show)",
  "\nShow options:",
  (.options.show | dump_options),
  "",
  (.options.help | dump_options),
  "",
  (.details.run | dump_details) ;

def list:
  "\(.program) version \(.version)",
  "\nUsage:",
  "  \(.program) \(.usage.list)",
  "    \(.text.list)",
  "\nList options:",
  (.options.list | dump_options),
  "",
  (.options.help | dump_options),
  "",
  (.details.list | dump_details) ;

def install:
  "\(.program) version \(.version)",
  "\nUsage:",
  "  \(.program) \(.usage.install)",
  "    \(.text.install)",
  "\nInstall options:",
  (.options.help | dump_options),
  "",
  (.details.install | dump_details) ;

def rebuild:
  "\(.program) version \(.version)",
  "\nUsage:",
  "  \(.program) \(.usage.rebuild)",
  "    \(.text.rebuild)",
  "\nRebuild options:",
  (.options.help | dump_options),
  "" ;

