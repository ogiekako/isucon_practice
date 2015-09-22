# -*- coding: utf-8 -*-
from __future__ import division, absolute_import, print_function, unicode_literals
from fabric.api import *

workload = 3

env.user = 'isucon'
env.key_filename = '~/.ssh/google_compute_engine'
env.roledefs = {
    'server': ['107.167.179.255'],
}

@roles('server')
def push():
    local('gox -osarch="linux/amd64" -output="go/golang-webapp" -rebuild ./go')

    sudo('supervisorctl stop isucon_go')
    put('go/golang-webapp', 'webapp/go/golang-webapp')
    put('go/templates/*', 'webapp/go/templates/')
    put('go/prepare/*', 'webapp/go/prepare/')
    put('sql/schema.sql', 'sql/schema.sql')
    put('init.sh', 'init.sh')

    run('chmod 755 webapp/go/golang-webapp')
    run('chmod 755 init.sh')
    sudo('supervisorctl start isucon_go')

    local('go build -o go/golang-webapp ./go/')

@roles('server')
def bench():
    run('./benchmarker bench --init=./init.sh --workload {}'.format(workload))
