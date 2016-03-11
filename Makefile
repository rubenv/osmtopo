.PHONY: all base dev binaries container push

all: container

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
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev bash -c "go get -tags=embed -v -t -d ./... && go install -tags=embed -v github.com/rubenv/osmtopo/osmtopo && go build -tags=embed -v -o tmp/osmtopo ./bin/osmtopo && go test -tags=embed -bench=. -benchmem -cover github.com/rubenv/osmtopo/..."

container: binaries
	cp Dockerfile-binaries tmp/Dockerfile
	docker build -t rubenv/osmtopo tmp/
	docker run -ti --rm rubenv/osmtopo osmtopo --help

push:
	docker push rubenv/osmtopo-base
	docker push rubenv/osmtopo-dev
	docker push rubenv/osmtopo
