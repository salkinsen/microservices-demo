#!/bin/bash
# use -s flag to skip specific services

. ./shared.sh

isElementInArray redis "${skipServices[@]}"
if [[ $? -ne 0  ]]; then services+=("redis:alpine"); fi


for i in "${services[@]}"
do
    echo "loading $i into cluster..."
    kind load docker-image $i
    if [ $? -ne 0 ]; then
        echo "failed to load $i"
        failedToLoad+=("$i")
    fi
done

if [ ${#failedToLoad[@]} -ne 0 ]; then
    echo "Failed to load: ${failedToLoad[@]}"
    exit 1
fi