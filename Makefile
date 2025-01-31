init:
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.39.0
	go get golang.org/x/tools/cmd/goimports
	go get -u github.com/golang/protobuf/protoc-gen-go@v1.3.2

lint:
	# goimports -local github.com/Laisky -w .
	go vet
	gofmt -s -w .
	go mod tidy
	golangci-lint run -c .golangci.lint.yml

changelog:
	./.scripts/generate_changelog.sh
