.PHONY: vendor
vendor:
	GO111MODULE=on go build -o ./bin/gofigure ./src/gofigure.go
	GO111MODULE=on go mod vendor && GO111MODULE=on go mod tidy