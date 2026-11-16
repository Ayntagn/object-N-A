module cyberwriter

go 1.27.3

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/ebitengine/purego v0.11.1
	github.com/egoist/mygo v0.8.3
	github.com/go-text/typesetting v0.3.5
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/image v0.46.0
	golang.org/x/net v0.59.0
	golang.org/x/sys v0.48.0
	golang.org/x/text v0.43.0
)

require github.com/dlclark/regexp2/v2 v2.2.1 // indirect

replace github.com/egoist/mygo => ./third_party/mygo
