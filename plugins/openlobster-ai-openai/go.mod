module github.com/neirth/openlobster/plugins/openlobster-ai-openai

go 1.25.0

require (
	github.com/openai/openai-go/v3 v3.30.0
	github.com/stealthrocket/net v0.2.1
)

require (
	github.com/neirth/openlobster/plugins/openlobster-sdk-base v0.0.0-00010101000000-000000000000
	github.com/tidwall/gjson v1.18.0 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tidwall/sjson v1.2.5 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/neirth/openlobster/plugins/openlobster-sdk-base => ../openlobster-sdk-base
