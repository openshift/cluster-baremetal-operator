#!/bin/bash

ALL_FILES=$(find . -name '*.go' ! -path '*/vendor/*' ! -path '*/.build/*')
BAD_FILES=$(gofmt -l ${ALL_FILES})

if [ -n "${BAD_FILES}" ]; then
    for f in $BAD_FILES; do
        echo $f
        gofmt -s -w $f
    done
    exit 1
fi
