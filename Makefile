all: valfs

valfs: 
	go build -o valfs main.go

run: valfs
	./valfs

test:
	go test ./...

clean:
	rm -f valfs

.PHONY: test run clean
