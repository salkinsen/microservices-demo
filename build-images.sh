#!/bin/bash
# use -s flag to skip specific services

. ./shared.sh

# echo "skipServices = ${skipServices[@]}"
# echo "services = ${services[@]}"

for i in "${services[@]}"
do
	docker build ./src/$i/ -t $i:latest || docker build ./src/$i/src/ -t $i
    if [ $? -ne 0 ]; then
        failedToBuild+=("$i")
    fi
done

if [ ${#failedToBuild[@]} -ne 0 ]; then
    echo "Failed to build: ${failedToBuild[@]}"
    exit 1
fi