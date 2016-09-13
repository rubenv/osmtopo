.PHONY: all clean

SOURCES=$(basename $(wildcard *.proto))
all: $(addsuffix .pb.go, $(SOURCES))

%.pb.go: %.proto
	protoc -I/usr/local/include -I. \
		-I$(GOPATH)/src \
		--gofast_out=. \
		$<

clean:
	rm -f *.pb.go *pb_test.go
