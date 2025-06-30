# Contributing guides

## Run Ztoperator locally

We use [flox](https://flox.dev/) to set up our local development environment.
To download flox you can download it from brew:
```shell
brew install --cask flox
```
or directly from their [website](https://flox.dev/docs/install-flox/).

You can then set up the development environment by running:
```shell
flox activate
```

### Local cluster and dependencies

We use [kind](https://kind.sigs.k8s.io/) as our local Kubernetes cluster. Run the following command to set up a local kind cluster and install 
the necessary dependencies:
```shell
make setup-local
```
This will set up a local kind cluster, install and apply Istio as service-mesh, and install and apply Ztoperator.  

### Run ztoperator

You can then run ztoperator by running:
```shell
make run-local
```

> [!TIP]
> If you wish to test the features of Ztoperator manually, you want to ensure that requests are routed to the pod through Istio servcie mesh. 
> One way to achieve this is to use [cloud-provider-kind](https://github.com/kubernetes-sigs/cloud-provider-kind) which sets up an external IP-address 
> that routes requests to Istio's configured load balancer.

## Running integration tests locally

To run the integration tests locally, you first need to set up a local Kubernetes cluster as explained in 
[earlier](#local-cluster-and-dependencies). 

You also need to install a mock OAuth2-provider to issue and validate OAuth2-tokens, 
and expose a test application which can be reached from outside the cluster. This can all be done by running:

```shell
make setup-local-test
```

Then run the tests:

- Run all tests without running ztoperator:
```shell
make run-test
```

- Run all tests with ztoperator running in the background:
```shell
make test
```

- Run specific tests. This will require you to run the ztoperator controller seperately in another terminal or in your IDE.
```shell
# Run ztoperator in IDE or on another terminal
make run-local

# Run specific test
make test-single dir=test/chainsaw/authpolicy/<TEST FOLDER>
```