#!/bin/bash
# use -s flag to skip specific services

services=( adservice cartservice checkoutservice currencyservice emailservice frontend loadgenerator paymentservice recommendationservice shippingservice)

usage() {
  printf "Usage: use -s flag to skip specific services, e.g $0 -s adservice -s emailservice\n"
}



# exits if elem is not found in array
removeFromServices()
{
    local elem="${1}"

    for i in "${!services[@]}"
    do
        if [[ "${services[i]}" == "$elem" ]]; then
            unset 'services[i]'
            # services has non-continuous indices now
            return 0
        fi
    done

    printf "Don't recognize "$OPTARG", maybe wrong spelling?\n"
    exit 1

}

while getopts 's:' flag; do
    case "${flag}" in
        s)  removeFromServices "$OPTARG"
            skipServices+=("$OPTARG")
            echo "Skipping "$OPTARG""
            ;;
        *)  usage
            exit 1 ;;
  esac
done


# echo "skipServices = ${skipServices[@]}"
# echo "services = ${services[@]}"

for i in "${services[@]}"
do
	docker build ./src/$i/ -t $i:latest || docker build ./src/$i/src/ -t $i:latest
    if [ $? -ne 0 ]; then
        failedToBuild+=("$i")
    fi
done

[ ${#failedToBuild[@]} -eq 0 ] || echo "Failed to build: ${failedToBuild[@]}"