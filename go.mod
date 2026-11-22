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
	golang.org/x/sys v0.50.0
	golang.org/x/text v0.44.0
	object-na.local/jpegscale v0.0.0
	object-na.local/localassets v0.0.0
	object-na.local/mdprojection v0.0.0
	object-na.local/pdfthumb v0.0.0
	object-na.local/pngscale v0.0.0
	object-na.local/shortsearch v0.0.0
	object-na.local/vaultdelta v0.0.0
	object-na.local/vectorsearch v0.0.0
)

require (
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/ncruces/go-sqlite3 v0.38.0 // indirect
	github.com/ncruces/go-sqlite3-wasm/v6 v6.7.35401 // indirect
	github.com/ncruces/julianday v1.0.0 // indirect
)

replace github.com/egoist/mygo => ./third_party/mygo-187

replace object-na.local/jpegscale => ./third_party/jpegscale

replace object-na.local/localassets => ./third_party/localassets

replace object-na.local/mdprojection => ./third_party/mdprojection

replace object-na.local/pdfthumb => ./third_party/pdfthumb

replace object-na.local/pngscale => ./third_party/pngscale

replace object-na.local/shortsearch => ./third_party/shortsearch

replace object-na.local/vaultdelta => ./third_party/vaultdelta

replace object-na.local/vectorsearch => ./third_party/vectorsearch
