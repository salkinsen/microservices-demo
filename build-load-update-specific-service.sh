#!/bin/bash

# builds, loads into kind cluster and restarts the specified microservice

ms=${1:?Please specify the microservice}

docker build ./src/$ms/ -t $ms || docker build ./src/$ms/src/ -t $ms
if [ $? -ne 0 ]; then
    echo "docker image could not be build, aborting ..."
    exit 1
fi

kind load docker-image $ms
if [ $? -ne 0 ]; then
    echo "docker image could not be loaded into kind cluster, aborting ..."
    exit 1
fi

kubectl rollout restart -f kubernetes-manifests/$ms.yaml