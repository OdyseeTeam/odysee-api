#!/bin/sh

ssh-keygen -t rsa -f token_privkey.rsa -m pem
./oapi db_migrate_up
./oapi
