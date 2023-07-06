# Ephemeral

Helm chart for the Carbyne Stack Ephemeral service to execute functions in a
serverless fashion.

## TL;DR

```bash
helm install ephemeral
```

## Introduction

This chart bootstraps an
[Ephemeral Service](https://github.com/carbynestack/ephemeral) deployment on a
[Kubernetes](http://kubernetes.io) cluster using the [Helm](https://helm.sh)
package manager.

> **Tip**: This chart is used in the `helmfile.d` based deployment specification
> available in the
> [`carbynestack/carbynestack`](https://github.com/carbynestack/carbynestack)
> repository.

## Prerequisites

- Kubernetes 1.18+ (may also work on earlier and later versions but has not been
  tested)

- In case you want to use Amphora secrets as inputs to your functions you need
  to deploy [Amphora](https://github.com/carbynestack/amphora) as well.

## Installing the Chart

To install the chart with the release name `my-release`, invoke

```bash
helm install --name my-release ephemeral
```

Make sure that your current working directory is `<project-base-dir>/charts`.
The command deploys Ephemeral on the Kubernetes cluster in the default
configuration. The [configuration](#configuration) section lists the parameters
that can be configured to customize the deployment.

> **Tip**: List all releases using `helm list`

## Uninstalling the Chart

To uninstall/delete the `my-release` deployment:

```bash
helm delete my-release
```

The command removes all the Kubernetes components associated with the chart and
deletes the release.

## Configuration

The following sections list the (main) configurable parameters of the
`ephemeral` chart and their default values. For the full list of configuration
parameters see `values.yaml`.

Specify each parameter using the `--set key=value[,key=value]` argument to
`helm install`. For example,

```bash
helm install --name my-release --set discovery.image.tag=<tag> ephemeral
```

The above command sets the Ephemeral Discovery Service image version to `<tag>`.

Alternatively, a YAML file that specifies the values for the parameters can be
provided while installing the chart. For example,

```bash
helm install --name my-release -f values.yaml ephemeral
```

### Global Parameters

| Parameter     | Description       | Default |
| ------------- | ----------------- | ------- |
| `playerCount` | Number of players | `2`     |

### Discovery Service

| Parameter                        | Description                                                                  | Default                            |
| -------------------------------- | ---------------------------------------------------------------------------- | ---------------------------------- |
| `discovery.image.registry`       | Image registry used to pull the Discovery Service image                      | `ghcr.io`                          |
| `discovery.image.repository`     | Discovery Image name                                                         | `carbynestack/ephemeral/discovery` |
| `discovery.image.tag`            | Discovery Image tag                                                          | `latest`                           |
| `discovery.image.pullPolicy`     | Discovery Image pull policy                                                  | `IfNotPresent`                     |
| `discovery.service.annotations`  | Annotations that should be attached to the Discovery service                 | `[]`                               |
| `discovery.frontendUrl`          | The external base URL of the VCP                                             | \`\`                               |
| `discovery.master.port`          | The port of the master discovery service instance                            | \`\`                               |
| `discovery.isMaster`             | Determines whether the service acts as master or slave                       | `true`                             |
| `discovery.slave.connectTimeout` | Timeout to establish the connection to the upstream master Discovery Service | `60s`                              |
| `discovery.stateTimeout`         | Timeout in which the transition to the next state is to be expected          | `60s`                              |
| `discovery.computationTimeout`   | Timeout in which the result of a gamee's mpc computation is to be expected   | `60s`                              |

### Network Controller

| Parameter                            | Description                                                      | Default                                     |
| ------------------------------------ | ---------------------------------------------------------------- | ------------------------------------------- |
| `networkController.image.registry`   | Image registry used to pull the Network Controller Service image | `ghcr.io`                                   |
| `networkController.image.repository` | Network Controller Image name                                    | `carbynestack/ephemeral/network-controller` |
| `networkController.image.tag`        | Network Controller Image tag                                     | `latest`                                    |
| `networkController.image.pullPolicy` | Network Controller Image pull policy                             | `IfNotPresent`                              |

### Ephemeral Service

| Parameter                                     | Description                                                               | Default                               |
| --------------------------------------------- | ------------------------------------------------------------------------- | ------------------------------------- |
| `ephemeral.knative.activation.timeoutSeconds` | Timout in seconds for the container to respond to the knative activation  | `3600`                                |
| `ephemeral.image.registry`                    | Image registry used to pull the Ephemeral Service image                   | `ghcr.io`                             |
| `ephemeral.image.repository`                  | Ephemeral Image name                                                      | `carbynestack/ephemeral/ephemeral`    |
| `ephemeral.image.tag`                         | Ephemeral Image tag                                                       | `latest`                              |
| `ephemeral.image.pullPolicy`                  | Ephemeral Image pull policy                                               | `IfNotPresent`                        |
| `ephemeral.service.annotations`               | Annotations that should be attached to the Ephemeral service              | `[]`                                  |
| `ephemeral.minScale`                          | The minimum amount of pods to keep alive for the application              | `1`                                   |
| `ephemeral.resources.requests.cpu`            | The requested CPU resources in CPU cores                                  | `100m`                                |
| `ephemeral.resources.requests.memory`         | The requested memory resources                                            | `256Mi`                               |
| `ephemeral.resources.limits.cpu`              | The CPU resource limit in CPU cores                                       | \`\`                                  |
| `ephemeral.amphora.host`                      | The hostname of the Amphora serivce                                       | `amphora`                             |
| `ephemeral.amphora.scheme`                    | The scheme used to access the Amphora serivce                             | `http`                                |
| `ephemeral.amphora.path`                      | The path under which the Amphora serivce is available                     | `/`                                   |
| `ephemeral.castor.host`                       | The hostname of the Castor serivce                                        | `castor`                              |
| `ephemeral.castor.scheme`                     | The scheme used to access the Castor serivce                              | `http`                                |
| `ephemeral.castor.path`                       | The path under which the Castor serivce is available                      | `/`                                   |
| `ephemeral.castor.tupleStock`                 | The number of tuples to hold in stock for each tuple type                 | `1000`                                |
| `ephemeral.discovery.host`                    | The host address of the discovery service                                 | `discovery.default.svc.cluster.local` |
| `ephemeral.discovery.port`                    | The port of the discovery service                                         | `8080`                                |
| `ephemeral.discovery.connectTimout`           | Timeout to establish the connection to the discovery service              | `60s`                                 |
| `ephemeral.frontendUrl`                       | The external base URL of the VCP                                          | \`\`                                  |
| `ephemeral.spdz.prime`                        | The prime used by SPDZ                                                    | \`\`                                  |
| `ephemeral.spdz.rInv`                         | The rInv used by SPDZ                                                     | \`\`                                  |
| `ephemeral.spdz.gfpMacKey`                    | The macKey for the prime protocol used by SPDZ                            | \`\`                                  |
| `ephemeral.spdz.gf2nMacKey`                   | The macKey for the GF(2^n) protocol used by SPDZ                          | \`\`                                  |
| `ephemeral.spdz.gf2nBitLength`                | The Bit length of the GF(2^n) field used by SPDZ                          | \`\`                                  |
| `ephemeral.spdz.gf2nStorageSize`              | The size of GF(2^n) tuples in bytes used by SPDZ                          | \`\`                                  |
| `ephemeral.spdz.prepFolder`                   | The directory where SPDZ expects the preprocessing data to be stored      | \`Player-Data\`                       |
| `ephemeral.playerId`                          | Id of this player                                                         | \`\`                                  |
| `ephemeral.networkEstablishTimeout`           | Timeout to establish network connections                                  | `1m`                                  |
| `ephemeral.player.stateTimeout`               | Timeout in which the transition to the next state is to be expected       | `60s`                                 |
| `ephemeral.player.computationTimeout`         | Timeout in which the result of a game's mpc computation is to be expected | `60s`                                 |
