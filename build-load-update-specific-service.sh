#!/bin/bash

# builds, loads into kind cluster and restarts the specified microservice

ms=${1:?Please specify the microservice}

docker build ./src/$ms/ -t $ms || docker build ./src/$ms/src/ -t $ms
kind load docker-image $ms
kubectl rollout restart -f kubernetes-manifests/$ms.yaml