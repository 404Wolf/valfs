ALL_GO_FILES := $(shell find . -type f -name '*.go')

all: valfs

valfs: $(ALL_GO_FILES)
	cd src && go build -o valfs main.go
	mv src/valfs .

run: valfs
	./valfs

test:
	go test ./...

clean:
	rm -f valfs

.PHONY: test run clean
