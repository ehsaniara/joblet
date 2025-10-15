module github.com/ehsaniara/joblet/persist

go 1.24.0

require (
	github.com/ehsaniara/joblet v0.0.0-00010101000000-000000000000
	github.com/ehsaniara/joblet-proto/v2 v2.2.1
	google.golang.org/grpc v1.76.0
	google.golang.org/protobuf v1.36.10
	gopkg.in/yaml.v3 v3.0.1
)

require (
	golang.org/x/net v0.44.0 // indirect
	golang.org/x/sys v0.36.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250922171735-9219d122eba9 // indirect
)

replace github.com/ehsaniara/joblet => ../
