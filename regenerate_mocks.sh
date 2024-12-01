#!/bin/bash
set -e -x

cd ./cast
go run github.com/vektra/mockery/v2@v2.49.1 --all

cd ../application
go run github.com/vektra/mockery/v2@v2.49.1 --all