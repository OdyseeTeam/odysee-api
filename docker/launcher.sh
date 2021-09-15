#!/bin/sh

ssh-keygen -t rsa -f token_privkey.rsa -m pem
./lbrytv db_migrate_up
./lbrytv
