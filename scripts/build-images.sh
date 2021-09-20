#!/bin/bash
# use -s flag to skip specific services

. ./shared.sh

for i in "${services[@]}"
do
	docker build ../src/$i/ -t salkinsen/$i || docker build ../src/$i/src/ -t salkinsen/$i
    if [ $? -ne 0 ]; then
        failedToBuild+=("$i")
    fi
done

if [ ${#failedToBuild[@]} -ne 0 ]; then
    echo "Failed to build: ${failedToBuild[@]}"
    exit 1
fi