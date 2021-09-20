#!/bin/bash

# this is a helper script that is used by the other scripts in this folder

services=( adservice cartservice checkoutservice currencyservice emailservice frontend paymentservice productcatalogservice recommendationservice shippingservice)


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

usage() {
  printf "Usage: use -s flag to skip specific services, e.g $0 -s adservice -s emailservice\n"
}

# first argument element, second array
isElementInArray () {
    local array="$1"
    local elem="$2"

    for i in "${array[@]}"
    do 
        if [[ "$i" == "$elem" ]]; then return 0; fi
    done

    return 1
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

