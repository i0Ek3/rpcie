.PHONY: build test clean

GO=go

build:
	@$(GO) build -o rpcie

test:
	@$(GO) test -v .

clean:
	@rm rpcie
