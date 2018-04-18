#!/bin/bash
ping -c 1 [HOST IP]
if [[ $? -ne 0 ]]; then
	echo "ping failed"
else
	echo "ping ok"
fi
