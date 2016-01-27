# -*- coding: utf-8 -*-
from __future__ import division, absolute_import, print_function, unicode_literals
from fabric.api import *

workload = 6

env.user = 'isucon'
env.key_filename = '~/.ssh/id_rsa'
env.roledefs = {
    'server': ['104.155.204.133'],
}

@roles('server')
def push():
    with lcd("/home/ogiekako/src/github.com/ogiekako/isucon_practice/isucon5_qual"):
        sudo('systemctl stop isuxi.go')
        local('go build -o /tmp/app ./go')
        put('go/templates/*', 'webapp/go/templates/')
        put('sql/*', 'webapp/sql/')
        put('static/*', 'webapp/static/')
        put('/tmp/app', 'webapp/go/app')
        run('chmod 755 webapp/go/app')
        sudo('systemctl start isuxi.go')

        bench()

@roles('server')
def bench():
    run('bench/bench.sh')
