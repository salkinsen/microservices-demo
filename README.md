This is a fork of Google's [**Online Boutique**](https://github.com/GoogleCloudPlatform/microservices-demo). In this fork, all microservices were instrumented with OpenTelemetry. Every gRPC call can be traced. This fork is part of my bachelor thesis **Resource Costs of Observability in Microservice Systems**. It is intended to be used with the [**Meta Monitoring Repository**](https://github.com/salkinsen/meta-monitoring), which includes instructions and scripts for deploying the microservices together with observability containers and the meta-monitoring-layer. Please look there for instructions on how to deploy the entire systems. Both repositories should be cloned into the same folder.

If you wish to deploy the microservices without observability and meta-monitoring-layer, this can be done via skaffold or kubectl, e.g.
```shell
kubectl apply -f kubernetes-manifests/microservices-no-tracing/
```
A load generator that produces 100 req/s can be deployed with
```shell
kubectl apply -f kubernetes-manifests/loadgenerator/loadgenerator100.yaml
```

Please note that the loadgenerator will only be deployed to a node with the [label](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/) layer:meta-monitoring. The microservices will **not** be deployed on such a node.

The [./scripts](./scripts)-folder contains Bash scripts that are intended to help with local development on a kind cluster.