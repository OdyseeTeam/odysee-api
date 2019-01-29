#!/bin/bash

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

(
  cd "$DIR"

  export DEBUGGING=1

  hash reflex 2>/dev/null || go get github.com/cespare/reflex
  hash reflex 2>/dev/null || { echo >&2 'Make sure '"$(go env GOPATH)"'/bin is in your $PATH'; exit 1;  }

  hash go-bindata 2>/dev/null || go get github.com/jteeuwen/go-bindata/...

  reflex --decoration=none --start-service=true --regex='\.(go|css|js|html)$' --inverse-regex='assets/bindata\.go' --inverse-regex='vendor/' -- sh -c "go generate && go run *.go serve"
)
