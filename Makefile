ALL_GO_FILES := $(shell find . -name '*.go')

all: build

build: $(ALL_GO_FILES)
	go build -gcflags="all=-N -l" -o build

run: build
	./build

test: build
	go test -v ./... -p 1

clean:
	rm -f build

.PHONY: all run test clean 
