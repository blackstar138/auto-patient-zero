#!/usr/bin/env bash
fail() {
    echo configuration failed
    exit 1
}

echo "--------------------------------------------------------------------"
echo -n "Setting up GO Environment Variables "
echo "--------------------------------------------------------------------"

export GOROOT=/usr/local/go || fail
export GOPATH=/home/mike/go || fail

echo "GOROOT and GOPATH correctly setup"
