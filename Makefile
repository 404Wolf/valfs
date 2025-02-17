ALL_GO_FILES := $(shell find . -type f -name '*.go')
all: build

build: $(ALL_GO_FILES)
	go build -gcflags="all=-N -l" -o build

run: build
	./build

test:
  # give it two chances, in case of unexpected API errors or ratelimits
	GOMAXPROCS=1 go test -v ./... -failfast || sleep 60 && go test -v ./... -run="$(FAILED)"

clean:
	rm -f build

.PHONY: all run test clean 
