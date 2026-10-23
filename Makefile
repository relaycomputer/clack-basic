.PHONY: all build clean

all: build

build:
	go build -o clack-basic main.go

clean:
	rm -rf clack-basic dist
