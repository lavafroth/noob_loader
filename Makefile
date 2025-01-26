BINARY_NAME=loader

build: 
	@go build -o bin/$(BINARY_NAME) .

run: build
	@./bin/$(BINARY_NAME)

clean:
	@rm -f bin/$(BINARY_NAME)

.PHONY: build run clean
