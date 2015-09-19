# -*- coding: utf-8 -*-
from __future__ import division, absolute_import, print_function, unicode_literals
from fabric.api import *

workload = 1

env.user = 'isucon'
env.key_filename = '~/.ssh/google_compute_engine'
env.roledefs = {
    'server': ['107.167.179.255'],
}

@roles('server')
def push():
    local('gox -osarch="linux/amd64" -output="go/golang-webapp" -rebuild ./go/')

    sudo('supervisorctl stop isucon_go')
    put('go/golang-webapp', 'webapp/go/golang-webapp')
    put('go/templates', 'webapp/go/templates')

    run('chmod 755 webapp/go/golang-webapp')
    sudo('supervisorctl start isucon_go')

    local('go build -o go/golang-webapp ./go/')

@roles('server')
def bench():
    run('./benchmarker bench --workload {}'.format(workload))
