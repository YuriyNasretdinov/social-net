#!/bin/sh -e -x
trap 'ssh -t freebsd "su -"' SIGINT

if ! ping -c 1 -t 1 192.168.56.2; then
	VBoxHeadless --startvm FreeBSD &
	sleep 60
	ssh freebsd uptime
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
