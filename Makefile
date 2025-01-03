ALL_GO_FILES := $(shell find . -type f -name '*.go')

all: valfs

valfs: $(ALL_GO_FILES)
	go build -gcflags="all=-N -l"

run: valfs
	./valfs

test:
	go test ./...

clean:
	rm -f valfs

.PHONY: test run clean
