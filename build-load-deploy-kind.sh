#!/bin/bash
# use -s flag to skip specific services

#!/bin/bash
# use -s flag to skip specific services

./build-images.sh "$@" 
if [ $? -ne 0 ]; then
    echo "not all images could be build, aborting ..."
    exit 1
fi

./load-to-kind.sh "$@"
if [ $? -ne 0 ]; then
    echo "not all images could be loaded into the kind cluster, aborting ..."
    exit 1
else
    echo "images succesfully loaded into kind cluster"
fi

./deploy-to-kind.sh "$@"    # sets pull policy to IfNotPresent
if [ $? -ne 0 ]; then
    echo "deployment failed"
    exit 1
fi