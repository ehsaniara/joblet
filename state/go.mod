module github.com/ehsaniara/joblet/state

go 1.24.0

require (
	github.com/aws/aws-sdk-go-v2 v1.39.3
	github.com/aws/aws-sdk-go-v2/config v1.31.12
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.18.9
	github.com/aws/aws-sdk-go-v2/service/dynamodb v1.50.3
	github.com/ehsaniara/joblet v0.0.0-00010101000000-000000000000
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aws/aws-sdk-go-v2/credentials v1.18.16 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.4.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.7.9 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.8.3 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/accept-encoding v1.13.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/endpoint-discovery v1.11.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.13.9 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.29.6 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.35.1 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.38.6 // indirect
	github.com/aws/smithy-go v1.23.1 // indirect
	github.com/maxbrunsfeld/counterfeiter/v6 v6.12.0 // indirect
	golang.org/x/mod v0.27.0 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	golang.org/x/tools v0.36.0 // indirect
)

replace github.com/ehsaniara/joblet => ../
