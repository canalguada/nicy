module github.com/canalguada/nicy

go 1.19

require (
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/mitchellh/go-homedir v1.1.0
	github.com/spf13/cobra v1.7.0
	github.com/spf13/viper v1.10.1
	golang.org/x/sys v0.8.0
	gopkg.in/yaml.v2 v2.4.0
	kernel.org/pub/linux/libs/security/libcap/cap v1.2.63
)

require (
	github.com/canalguada/procfs v0.0.0-unpublished // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/hashicorp/hcl v1.0.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.2.0 // indirect
	github.com/magiconair/properties v1.8.5 // indirect
	github.com/mitchellh/mapstructure v1.4.3 // indirect
	github.com/pelletier/go-toml v1.9.4 // indirect
	github.com/spf13/afero v1.6.0 // indirect
	github.com/spf13/cast v1.4.1 // indirect
	github.com/spf13/jwalterweatherman v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/subosito/gotenv v1.2.0 // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/check.v1 v1.0.0-20190902080502-41f04d3bba15 // indirect
	gopkg.in/ini.v1 v1.66.2 // indirect
	kernel.org/pub/linux/libs/security/libcap/psx v1.2.63 // indirect
)

replace github.com/canalguada/procfs v0.0.0-unpublished => ./procfs
