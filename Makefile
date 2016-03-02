.PHONY: all dev binaries container

all: container

base: Dockerfile-base
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-base tmp/Dockerfile
	docker build -t rubenv/osmtopo-base tmp/

dev: base Dockerfile-dev
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-dev tmp/Dockerfile
	docker build -t rubenv/osmtopo-dev tmp/

binaries: base dev
	rm -rf tmp/
	mkdir -p tmp/
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev bash scripts/build-binaries.sh

container: binaries
	cp Dockerfile-binaries tmp/Dockerfile
	docker build -t rubenv/osmtopo tmp/
