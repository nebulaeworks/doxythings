# Developer shortcuts. The binary itself needs only "go build".

.PHONY: build test docs clean

# build compiles doxythings into bin/.
build:
	go build -o bin/doxythings ./cmd/doxythings

# test runs every test; the end-to-end test needs doxygen on PATH.
test:
	go vet ./...
	go test ./...

# DOXYGO is the doxygo filter that lets Doxygen read Go
# (github.com/nebulaeworks/doxygo): "go install" it, or point this at a
# checkout's binary, e.g. make docs DOXYGO=../doxygo/bin/doxygo.
DOXYGO ?= $(shell command -v doxygo)

# docs builds this repository's own Doxygen documentation into build/docs.
docs:
	@test -n "$(DOXYGO)" || { echo "doxygo not found: go install github.com/nebulaeworks/doxygo@latest, or set DOXYGO"; exit 1; }
	mkdir -p build/docs
	DOXYGO=$(abspath $(DOXYGO)) doxygen Doxyfile

# clean removes build output.
clean:
	rm -rf bin build
