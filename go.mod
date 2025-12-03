module github.com/ehsaniara/joblet

go 1.24.0

require (
	github.com/ehsaniara/joblet-proto/v2 v2.4.0
	github.com/spf13/cobra v1.10.1
	github.com/spf13/pflag v1.0.10
	github.com/stretchr/testify v1.11.1
	google.golang.org/grpc v1.77.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.12.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/mod v0.29.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/sync v0.18.0 // indirect
	golang.org/x/sys v0.38.0 // indirect
	golang.org/x/text v0.31.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251111163417-95abcf5c77ba // indirect
)

tool github.com/maxbrunsfeld/counterfeiter/v6

// Workaround for counterfeiter issue #344: VendorlessPath removed in x/tools v0.38.0
// Remove this when counterfeiter releases a fix
replace golang.org/x/tools => golang.org/x/tools v0.37.0
