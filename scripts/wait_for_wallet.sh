#!/bin/bash

echo "Waiting for wallet server '${LW_LBRYNET}'..."

while [[ ! $(curl -sd '{"method": "status"}' ${LW_LBRYNET} |grep '"wallet": true') ]]; do
    sleep 1
done


echo "Wallet server is alive"
