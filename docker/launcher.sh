#!/bin/sh

file="token_privkey.rsa"
if [ ! -f "$file" ]; then
    ssh-keygen -t rsa -f "$file" -m pem
fi

./oapi db_migrate_up
./oapi
