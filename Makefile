.PHONY: be-build be-test be-lint fe-build fe-test build test lint

build: be-build fe-build

test: be-test fe-test

lint: be-lint

be-build:
	$(MAKE) -C be build

be-test:
	$(MAKE) -C be test

be-lint:
	$(MAKE) -C be lint

fe-build:
	$(MAKE) -C fe build

fe-test:
	$(MAKE) -C fe test
