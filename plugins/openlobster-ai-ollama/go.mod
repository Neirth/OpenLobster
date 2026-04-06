module github.com/neirth/openlobster/plugins/openlobster-ai-ollama

go 1.25.0

require (
	github.com/ollama/ollama v0.19.0
	github.com/stealthrocket/net v0.2.1
	golang.org/x/crypto v0.49.0
)

require (
	github.com/bahlo/generic-list-go v0.2.0 // indirect
	github.com/buger/jsonparser v1.1.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/neirth/openlobster/plugins/openlobster-sdk-base v0.0.0-00010101000000-000000000000
	github.com/wk8/go-ordered-map/v2 v2.1.8 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/neirth/openlobster/plugins/openlobster-sdk-base => ../openlobster-sdk-base
