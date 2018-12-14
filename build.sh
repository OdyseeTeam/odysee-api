#!/bin/bash

set -euo pipefail

DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

(
  cd "$DIR"

  if ! diff <(echo -n) <(find . -maxdepth 1 ! -path . -type d | grep -v vendor | xargs gofmt -l); then
    echo "Some files are not formatted correctly. Run 'go fmt' before committing code."
    exit 1
  fi

  importpath="github.com/lbryio/lbryweb.go"
  go generate -v
  if ! git diff --exit-code; then
    # Check that go generate produces a zero diff
    echo "Run 'go generate' before committing code."
    exit 1
  fi
  VERSION="${TRAVIS_COMMIT:-"$(git describe --always --dirty --long)"}"
  COMMIT_MSG="$(echo ${TRAVIS_COMMIT_MESSAGE:-"$(git show -s --format=%s)"} | tr -d '"' | head -n 1)"
  CGO_ENABLED=0 go build -v -o "lbryweb" -asmflags -trimpath="$DIR" -ldflags "-X ${importpath}/meta.version=${VERSION} -X \"${importpath}/meta.commitMessage=${COMMIT_MSG}\""
)

exit 0