.PHONY: build-ApiFunction

build-ApiFunction:
	# Build with CGO enabled for SQLite/lrcat-go support
	# SAM's provided.al2023 build image includes gcc and sqlite
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags lambda.norpc -o $(ARTIFACTS_DIR)/bootstrap main.go
