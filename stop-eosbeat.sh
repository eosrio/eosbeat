#!/bin/bash

if [ -f "eosbeat.pid" ]; then
	pid=`cat "eosbeat.pid"`
	kill -9 $pid
	rm -r "eosbeat.pid"
	echo -ne "Stopping eosbeat"
        while true; do
            [ ! -d "/proc/$pid/fd" ] && break
            echo -ne "."
            sleep 1
        done
        echo -ne "\reosbeat stopped!    \n"
    fi
