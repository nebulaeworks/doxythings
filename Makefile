# Developer shortcuts. The binary itself needs only "go build".

.PHONY: build test docs clean

# build compiles doxythings into bin/.
build:
	go build -o bin/doxythings ./cmd/doxythings

# test runs every test; the end-to-end test needs doxygen on PATH.
test:
	go vet ./...
	go test ./...

# docs builds this repository's own Doxygen documentation into build/docs.
docs:
	go build -o bin/doxygo ./tools/doxygo
	mkdir -p build/docs
	doxygen Doxyfile

# clean removes build output.
clean:
	rm -rf bin build
