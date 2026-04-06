module github.com/neirth/openlobster/plugins/openlobster-messages-twilio

go 1.25.0

require (
	github.com/stealthrocket/net v0.2.1
	github.com/twilio/twilio-go v1.30.4
)

require (
	github.com/golang-jwt/jwt/v5 v5.2.2 // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/neirth/openlobster/plugins/openlobster-sdk-base v0.0.0-00010101000000-000000000000
	github.com/pkg/errors v0.9.1 // indirect
	golang.org/x/net v0.51.0 // indirect
	golang.org/x/sys v0.42.0 // indirect
	golang.org/x/text v0.35.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250715232539-7130f93afb79 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/neirth/openlobster/plugins/openlobster-sdk-base => ../openlobster-sdk-base
