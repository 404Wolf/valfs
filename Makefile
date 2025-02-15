ALL_GO_FILES := $(shell find . -type f -name '*.go')
all: valfs

valfs: $(ALL_GO_FILES)
	go build -gcflags="all=-N -l"

run: valfs
	./valfs

test:
  # give it two chances, in case of unexpected API errors or ratelimits
	GOMAXPROCS=1 go test -v ./... -failfast || sleep 60 && go test -v ./... -run="$(FAILED)"

clean:
	rm -f valfs

.PHONY: all run test clean 
