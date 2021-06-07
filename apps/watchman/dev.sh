#!/bin/bash

TOKEN=$(docker exec influxdb influx auth list --hide-headers|awk -F '\t' 'NR==1{print $3}')

cat << EOF > watchman.yaml
InfluxDB:
  Token: ${TOKEN}
  URL: http://localhost:8086
  Org: odysee
  Bucket: watchman
EOF
