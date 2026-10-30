module github.com/wargasipil/san_tower

go 1.26.0

require (
	cel.dev/cel-go v0.32.0
	connectrpc.com/connect v1.21.1
	github.com/glebarez/sqlite v1.11.0
	github.com/mstrYoda/goraphdb v0.0.0-20260220134623-01eff165f35c
	github.com/robfig/cron/v3 v3.0.1
	github.com/urfave/cli/v3 v3.14.0
	github.com/wargasipil/san_message v0.0.0
	github.com/wargasipil/san_remote v0.0.0-00010101000000-000000000000
	github.com/yuin/goldmark v1.8.6
	google.golang.org/protobuf v1.36.12
	gorm.io/gorm v1.31.2
)

require (
	codeberg.org/readeck/go-readability/v2 v2.1.2 // indirect
	github.com/JohannesKaufmann/dom v0.3.1 // indirect
	github.com/JohannesKaufmann/html-to-markdown/v2 v2.5.2 // indirect
	github.com/andybalholm/cascadia v1.3.4 // indirect
	github.com/go-shiori/dom v0.0.0-20230515143342-73569d674e1c // indirect
	github.com/gogs/chardet v0.0.0-20211120154057-b7413eaefb8f // indirect
	github.com/itlightning/dateparse v0.2.1 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.etcd.io/bbolt v1.4.0 // indirect
)

require (
	cel.dev/expr v0.25.1 // indirect
	filippo.io/edwards25519 v1.2.0 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.1 // indirect
	github.com/beeper/argo-go v1.1.2 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/edsrzf/mmap-go v1.2.0 // indirect
	github.com/elliotchance/orderedmap/v3 v3.1.0 // indirect
	github.com/fsnotify/fsnotify v1.9.0
	github.com/glebarez/go-sqlite v1.21.2
	github.com/google/uuid v1.6.0 // indirect
	github.com/jinzhu/inflection v1.0.0 // indirect
	github.com/jinzhu/now v1.1.5 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/petermattis/goid v0.0.0-20260918085751-abfca077860b // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
	github.com/vektah/gqlparser/v2 v2.5.27 // indirect
	go.mau.fi/libsignal v0.2.2 // indirect
	go.mau.fi/util v0.11.0 // indirect
	go.mau.fi/whatsmeow v0.0.0-20261026125351-1fbae988179a
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/crypto v0.58.0 // indirect
	golang.org/x/exp v0.0.0-20261014143641-f0251eb4aa3e // indirect
	golang.org/x/net v0.61.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20240826202546-f6391c0de4c7 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251029180050-ab9386a59fda // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	modernc.org/libc v1.22.5 // indirect
	modernc.org/mathutil v1.5.0 // indirect
	modernc.org/memory v1.5.0 // indirect
	modernc.org/sqlite v1.23.1 // indirect
	san_knowledge v0.0.0
)

replace github.com/wargasipil/san_message => ../../packages/san_message

replace github.com/wargasipil/san_remote => ../san_remote

replace san_knowledge => ../../packages/san_knowledge
