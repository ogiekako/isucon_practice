# -*- coding: utf-8 -*-
from __future__ import division, absolute_import, print_function, unicode_literals
from fabric.api import *

workload = 5

env.user = 'isucon'
env.key_filename = '~/.ssh/google_compute_engine'
env.roledefs = {
    'server': ['107.167.179.255'],
}

@roles('server')
def push():
    sudo('supervisorctl stop isucon_go')
    local('gox -osarch="linux/amd64" -output="/tmp/golang-webapp" -rebuild ./go')
    put('go/templates/*', 'webapp/go/templates/')
    put('/tmp/golang-webapp', 'webapp/go/golang-webapp')
    run('chmod 755 webapp/go/golang-webapp')
    sudo('supervisorctl start isucon_go')

    local('gox -osarch="linux/amd64" -output="/tmp/golang-prepare" -rebuild ./go/prepare')
    put('/tmp/golang-prepare', 'webapp/go/golang-prepare')
    put('sql/schema.sql', 'sql/schema.sql')
    put('init.sh', 'init.sh')
    run('chmod 755 webapp/go/golang-prepare')
    run('chmod 755 init.sh')
    bench()

@roles('server')
def bench():
    run('./benchmarker bench --init=./init.sh --workload {}'.format(workload))
