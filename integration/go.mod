module go.abhg.dev/doc2go/integration

go 1.25.0

replace go.abhg.dev/doc2go => ../

require (
	github.com/andybalholm/cascadia v1.3.4
	github.com/stretchr/testify v1.12.1
	go.abhg.dev/container/ring v0.3.0
	go.abhg.dev/doc2go v0.12.2
	golang.org/x/net v0.58.0
)

require go.yaml.in/yaml/v3 v3.0.5 // indirect
