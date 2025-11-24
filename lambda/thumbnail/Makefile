.PHONY: build-ThumbnailFunction

build-ThumbnailFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o $(ARTIFACTS_DIR)/bootstrap main.go
