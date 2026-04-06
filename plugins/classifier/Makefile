PLUGIN_NAME=classifier
VERSION=v1.0.0
GO_VERSION=1.26.1

.PHONY: build build-docker clean

build:
	go build -v -buildmode=plugin -o $(PLUGIN_NAME).so main.go

build-docker:
	docker run --rm --platform linux/amd64 -v $(CURDIR):/app -w /app golang:$(GO_VERSION) \
		sh -c "export CGO_ENABLED=1 && go mod tidy && go build -v -buildmode=plugin -o $(PLUGIN_NAME).so main.go"

clean:
	rm -f $(PLUGIN_NAME).so
