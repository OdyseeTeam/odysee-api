#!/bin/bash

while [[ ! $(curl -sd '{"method": "status"}' $1 |grep '"wallet": true') ]]; do
    sleep 1
done
