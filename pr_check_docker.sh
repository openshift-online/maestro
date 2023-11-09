#!/bin/bash -ex

# Reflects defaults found in pkg/config/db.go
export GORM_DIALECT="postgres"
export GORM_HOST="localhost"
export GORM_PORT="5432"
export GORM_NAME="maestro"
export GORM_USERNAME="maestro"
export GORM_PASSWORD="foobar-bizz-buzz"
export GORM_SSLMODE="disable"
export GORM_DEBUG="false"

export LOGLEVEL="1"
export TEST_SUMMARY_FORMAT="standard-verbose"

go version
mkdir "$(go env GOPATH)/bin"
which gotestsum || curl -sSL "https://github.com/gotestyourself/gotestsum/releases/download/v0.3.5/gotestsum_0.3.5_linux_amd64.tar.gz" | tar -xz -C "$(go env GOPATH)/bin" gotestsum

which pg_ctl
PGDATA=/var/lib/postgresql/data /usr/lib/postgresql/*/bin/pg_ctl -w stop
PGDATA=/var/lib/postgresql/data /usr/lib/postgresql/*/bin/pg_ctl start -o "-c listen_addresses='*' -p 5432"

make test
make test-integration

exit 0
