#!/bin/sh

go mod verify
go mod tidy
go mod vendor
