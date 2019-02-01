#!/bin/bash

while [[ ! $(curl -sd '{"method": "status"}' ${LW_LBRYNET} |grep '"wallet": true') ]]; do
    sleep 1
done
