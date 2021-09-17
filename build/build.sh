#!/bin/sh

set -o errexit
set -o nounset

if [ ! -f "build/build.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

if [ -z "${PKG}" ]; then
    echo "PKG must be set"
    exit 1
fi
if [ -z "${ARCH}" ]; then
    echo "ARCH must be set"
    exit 1
fi
if [ -z "${VERSION}" ]; then
    echo "VERSION must be set"
    exit 1
fi



export CGO_ENABLED=0
export GOARCH="${ARCH}"
export GOBIN=$PWD"/bin/${ARCH}/"


unameOut="$(uname -s)"
case "${unameOut}" in
    Linux*)     machine=Linux;;
    Darwin*)    machine=Mac;;
    CYGWIN*)    machine=Cygwin;;
    MINGW*)     machine=MinGw;;
    *)          machine="UNKNOWN:${unameOut}"
esac

#Make file path correct for windows system.
if [ "$machine" = "Cygwin" ]; then
    echo "Detected cygwin, converting GOBIN path to windows format (/cygdrive/c/... -> C:\...)"
    export GOBIN=$(cygpath -w "$GOBIN")
fi

go install                                              \
    -installsuffix "static"                             \
    -ldflags "-X ${PKG}/pkg/version.VERSION=${VERSION}" \
    ./...
