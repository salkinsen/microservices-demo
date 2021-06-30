## build all images, load them to kind cluster and deploy
```shell
./build-load-deploy-kind.sh
```
to skip services, use -s tag, e.g.
```shell
./build-load-deploy-kind.sh -s recommendationservice -s emailservice
```

## build, load to kind and restart specific service
```shell
./build-load-update-specific-service.sh <service name>
```
e.g.  
```shell
./build-load-update-specific-service.sh adservice
```