module github.com/neirth/openlobster/plugins/openlobster-memory-gml

go 1.25.0

require github.com/stretchr/testify v1.11.1

require (
	golang.org/x/net v0.38.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250324211829-b45e905df463 // indirect
	google.golang.org/grpc v1.73.0 // indirect
	google.golang.org/protobuf v1.36.6 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/neirth/openlobster/plugins/openlobster-sdk-base v0.0.0-00010101000000-000000000000
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/neirth/openlobster/plugins/openlobster-sdk-base => ../openlobster-sdk-base
