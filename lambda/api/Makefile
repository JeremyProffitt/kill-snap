.PHONY: build clean

build:
	go mod download
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap main.go
	zip deployment.zip bootstrap

clean:
	rm -f bootstrap deployment.zip
