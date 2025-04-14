#!/bin/bash -ex

cd ./cast
go run github.com/vektra/mockery/v2@v2.53.3

cd ../application
go run github.com/vektra/mockery/v2@v2.53.3
