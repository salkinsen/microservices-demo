#!/bin/bash
# use -s flag to skip specific services

. ./shared.sh


for i in "${services[@]}"
do
	docker push salkinsen/$i:latest
    if [ $? -ne 0 ]; then
        failedToPush+=("$i")
    fi
done

if [ ${#failedToPush[@]} -ne 0 ]; then
    echo "Failed to push images: ${failedToPush[@]}"
    exit 1
fi