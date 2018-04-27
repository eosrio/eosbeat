#!/bin/bash

if [ -f "eosbeat.pid" ]; then
	echo "eosbeat is already running!"
	pid=`cat "eosbeat.pid"`
	read -p "Do you want to restart? [y / n]" -n 1 -r
	echo -e "\n"
	if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    		[[ "$0" = "$BASH_SOURCE" ]] && exit 1 || return 1 # handle exits from shell or function but don't exit interactive shell
	fi
	./stop-eosbeat.sh
fi
echo -e "Starting eosbeat..."
./eosbeat -strict.perms=false > out.log 2> err.log & echo $! > eosbeat.pid
