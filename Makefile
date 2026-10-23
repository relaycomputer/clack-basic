.PHONY: all build clean test

all: build

build:
	go build -o clack-basic main.go

test:
	go test -v ./pkg/...

clean:
	rm -rf clack-basic dist
