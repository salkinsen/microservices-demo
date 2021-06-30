#!/bin/bash
# use -s flag to skip specific services

. ./shared.sh


cleanup()
{
    for i in "${deployedServices[@]}"
    do
        kubectl delete -f ./kubernetes-manifests/$i.yaml 
    done
}

isElementInArray redis "${skipServices[@]}"
if [[ $? -ne 0  ]]; then services+=("redis"); fi

for i in "${services[@]}"
do
    kubectl apply -f ./kubernetes-manifests/$i.yaml
    deployedServices+=("$i")

    if [ $? -ne 0 ]; then
        echo "failed to deploy $i, cleaning up..."
        cleanup
        exit 1
    fi
done