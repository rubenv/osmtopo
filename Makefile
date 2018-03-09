.PHONY: all base dev binaries container push

IMG=docker.io/rubenv/osmtopo

all: container

base: Dockerfile-base
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-base tmp/Dockerfile
	docker pull $(IMG)-base
	docker build --pull -t $(IMG)-base tmp/

dev: base Dockerfile-dev
	rm -rf tmp/
	mkdir -p tmp/
	cp Dockerfile-dev tmp/Dockerfile
	docker pull $(IMG)-dev
	docker build -t $(IMG)-dev tmp/

binaries: base dev
	rm -rf tmp/
	mkdir -p tmp/
	docker run -ti --rm -v $(GOPATH)/src:/go/src:Z $(IMG)-dev bash -c "go get -v -t -d ./... && go install -x -v github.com/rubenv/osmtopo/osmtopo && go build -v -o tmp/osmtopo ./bin/osmtopo && go test -bench=. -benchmem -cover github.com/rubenv/osmtopo/..."

container: binaries
	cp Dockerfile-binaries tmp/Dockerfile
	docker build -t $(IMG) tmp/
	docker run -ti --rm $(IMG) osmtopo --help

pull:
	docker pull $(IMG)-base
	docker pull $(IMG)-dev
	docker pull $(IMG)

push:
	docker push $(IMG)-base
	docker push $(IMG)-dev
	docker push $(IMG)
