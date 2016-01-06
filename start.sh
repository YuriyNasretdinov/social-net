#!/bin/sh -e -x
trap "mysql.server stop; killall memcached" SIGINT

if ! echo version | nc localhost 11211; then
    mysql.server start
    memcached -d
fi

go build

URL='http://localhost:8080'

(
sleep 2
open -a Firefox "$URL"
open -a 'Google Chrome' "$URL"
open -a Safari "$URL"
)

php rebuilder.php
