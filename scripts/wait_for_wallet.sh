#!/bin/bash

lbrynet=http://localhost:5279/
echo "Waiting for wallet server '${lbrynet}'..."

while [[ ! $(curl -sd '{"method": "status"}' ${lbrynet} |grep '"wallet": true') ]]; do
    sleep 1
done


echo "Wallet server is alive"
