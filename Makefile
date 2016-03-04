.PHONY: all dev binaries container test push

all: container test

base: Dockerfile-base
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-base tmp/Dockerfile
	docker pull fedora:23
	docker pull rubenv/osmtopo-base
	docker build -t rubenv/osmtopo-base tmp/

dev: base Dockerfile-dev
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-dev tmp/Dockerfile
	docker pull rubenv/osmtopo-dev
	docker build -t rubenv/osmtopo-dev tmp/

binaries: base dev
	rm -rf tmp/
	mkdir -p tmp/
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev go get -v -t ./...
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev go build -v -o tmp/osmtopo ./bin/osmtopo

container: binaries
	cp Dockerfile-binaries tmp/Dockerfile
	docker build -t rubenv/osmtopo tmp/

test: binaries
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev go test -bench=. -benchmem -cover github.com/rubenv/osmtopo/...

push:
	docker push rubenv/osmtopo-base
	docker push rubenv/osmtopo-dev
	docker push rubenv/osmtopo
