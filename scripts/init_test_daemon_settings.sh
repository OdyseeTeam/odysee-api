#!/bin/bash

cat > /storage/data/daemon_settings.yml <<EOF
lbryum_servers:
  - orchstr8:50001

blockchain_name: lbrycrd_regtest

data_dir: /storage/data
download_directory: /storage/downloads
wallet_dir: /storage/lbryum

delete_blobs_on_remove: True
use_upnp: False
EOF

exec "$@"

