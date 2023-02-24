all: reformat heapspurs

SOURCE=$(shell find * -name '*.go')

heapspurs: $(SOURCE)
	go build ./cmd/heapspurs

reformat:
	find . -name '*.go' -exec gofmt -s -w '{}' \+
