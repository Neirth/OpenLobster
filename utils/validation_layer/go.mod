module github.com/neirth/openlobster/utils/validation_layer

go 1.24

require (
	github.com/google/uuid v1.6.0
	github.com/neirth/openlobster/utils/schema_contract v0.0.0
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/neirth/openlobster/utils/schema_contract => ../schema_contract
