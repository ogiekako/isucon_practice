#!/bin/bash
export GOPATH=~/private/src

go get github.com/go-martini/martini
go get github.com/go-sql-driver/mysql
go get github.com/martini-contrib/render
go get github.com/martini-contrib/sessions
go get github.com/draftcode/isucon_misc/grizzly
go build -o golang-webapp .
