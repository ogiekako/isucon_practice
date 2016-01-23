# -*- coding: utf-8 -*-
from __future__ import division, absolute_import, print_function, unicode_literals
from fabric.api import *

workload = 6

env.user = 'isucon'
env.key_filename = '~/.ssh/id_rsa'
env.roledefs = {
    'server': ['104.155.216.150'],
}

@roles('server')
def push():
    sudo('systemctl stop isuxi.go.server')
    local('go build -o /tmp/app ./go')
    put('go/templates/*', 'webapp/go/templates/')
    put('sql/*', 'webapp/sql/')
    put('static/*', 'webapp/static/')
    put('/tmp/app', 'webapp/go/app')
    run('chmod 755 webapp/go/app')
    sudo('systemctl start isuxi.go.server')

    bench()

@roles('server')
def bench():
    with cd('bench'):
        run('jq \'.[0]\' < ../webapp/script/testsets/testsets.json | gradle run -Pargs="net.isucon.isucon5q.bench.scenario.Isucon5Qualification localhost:8080"')
