#!/bin/sh -e -x
VMNAME="FreeBSD"
HOST="socialsrv"

trap "VBoxManage controlvm $VMNAME acpipowerbutton" SIGINT

if ! ping -c 1 -t 1 $HOST; then
	VBoxHeadless --startvm $VMNAME &
	sleep 60
	ssh $HOST uptime
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
