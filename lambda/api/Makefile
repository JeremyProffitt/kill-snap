.PHONY: build-ApiFunction

build-ApiFunction:
	# Pure-Go build (no cgo dependencies in source); targets arm64/Graviton
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o $(ARTIFACTS_DIR)/bootstrap main.go
