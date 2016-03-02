#!/bin/bash

cd "$(dirname "$0")/../bin/" || exit 1

rm -rf ../tmp/
mkdir -p ../tmp/

go get -v -t ../...
go install -v ../
for f in *; do
    go build -v -o "../tmp/$f" "./$f"
done
