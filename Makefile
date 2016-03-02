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
	cp Dockerfile-binaries tmp/Dockerfile
	docker run -ti --rm -v /go:/go rubenv/osmtopo-dev bash scripts/build-binaries.sh

container: binaries
	docker build -t rubenv/osmtopo tmp/
