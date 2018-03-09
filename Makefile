.PHONY: all base dev binaries container push

all: container

base: Dockerfile-base
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-base tmp/Dockerfile
	docker pull rubenv/osmtopo-base
	docker build --pull -t rubenv/osmtopo-base tmp/

dev: base Dockerfile-dev
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-dev tmp/Dockerfile
	docker pull rubenv/osmtopo-dev
	docker build --pull -t rubenv/osmtopo-dev tmp/

binaries: base dev
	rm -rf tmp/
	mkdir -p tmp/
	docker run -ti --rm -v $(GOPATH)/src:/go/src rubenv/osmtopo-dev bash -c "go get -v -t -d ./... && go install -x -v github.com/rubenv/osmtopo/osmtopo && go build -v -o tmp/osmtopo ./bin/osmtopo && go test -bench=. -benchmem -cover github.com/rubenv/osmtopo/..."

container: binaries
	cp Dockerfile-binaries tmp/Dockerfile
	docker build --pull -t rubenv/osmtopo tmp/
	docker run -ti --rm rubenv/osmtopo osmtopo --help

push:
	docker push rubenv/osmtopo-base
	docker push rubenv/osmtopo-dev
	docker push rubenv/osmtopo
