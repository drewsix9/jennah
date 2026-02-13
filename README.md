## Overview

`jennah` is an opinionated, workload deployment platform for the cloud. Internal to Alphaus.


### Development guidelines

#### Generate code from proto files:
```bash
   $ make generate
```

#### Build the gateway binary and run:
```bash
  $ cd cmd/gateway
  $ make build
  $ ./bin/gateway -h
```
#### Build gateway Docker image:
```bash
   $ make gw-docker-build
```
#### Run gateway container:
```bash
   $ make gw-docker-run
```
#### Push gateway to registry:
```bash
   $ make gw-docker-push
```

#### Notes:
Make sure to run the command below everytime you have made changes especially when adding/removing packages.
```bash
  $ go mod vendor && go mod tidy
```
 