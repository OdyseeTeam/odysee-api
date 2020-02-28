#!/bin/bash

set -o xtrace


if [ -z ${BASE_IMAGE+x} ]; then
    echo "Please provide base docker image name as BASE_IMAGE variable."
    exit 1
fi

TYPE=$1
NAME=$2
VERSION=`git describe --tags`
REASON="Automatic update to ${VERSION}"

if [ $TYPE == "sset" ]
then
    URL="/apis/apps/v1/namespaces/default/statefulsets/"

elif [ $TYPE == "deployment" ]
then
    URL="/apis/extensions/v1beta1/namespaces/default/deployments/"

else
    echo "No idea what ${TYPE} is, should be 'sset' or 'deployment"
fi

echo $REASON

curl -X PATCH -H 'Content-Type: application/strategic-merge-patch+json' --data '
{"spec":{"template":{"metadata": {"annotations": {"kubernetes.io/change-cause": "'"$REASON"'"}}, "spec":{"containers":[{"name":"api","image":"'"$BASE_IMAGE:$VERSION"'"}]}}}}' \
	${K8_APISERVER_PROD}${URL}${NAME} --header "Authorization: Bearer $K8_TOKEN_PROD" --insecure
