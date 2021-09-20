The Bash scripts `build-images.sh` and `push-images.sh` can be used to build all images and push them to Docker Hub. The rest of the scripts are intended to be used with a kind cluster. By default they apply the manifests from the [../kubernetes-manifests/microservices-no-tracing](../kubernetes-manifests/microservices-no-tracing) folder (meaning tracing instrumentation is disabled and the loadgenerator is not deployed).

### build all images, load them to kind cluster and deploy
```shell
./build-load-deploy-kind.sh
```
to skip services, use -s tag, e.g.
```shell
./build-load-deploy-kind.sh -s recommendationservice -s emailservice
```

### build, load to kind and restart specific service
```shell
./build-load-update-specific-service.sh <service name>
```
e.g.  
```shell
./build-load-update-specific-service.sh adservice
```