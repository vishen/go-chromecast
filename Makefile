all: deps build

build:
	go build -o chromecast .

deps:
	glide install

run_local:
	go run *.go
