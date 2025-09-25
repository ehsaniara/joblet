package api

// Generate protocol buffer code from joblet-proto module
//go:generate sh -c "rm -rf gen && mkdir -p gen"
//go:generate sh -c "protoc --proto_path=$(go env GOMODCACHE)/github.com/ehsaniara/joblet-proto@v1.0.2/proto --go_out=gen --go_opt=paths=source_relative --go-grpc_out=gen --go-grpc_opt=paths=source_relative $(go env GOMODCACHE)/github.com/ehsaniara/joblet-proto@v1.0.2/proto/joblet.proto"
