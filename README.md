# Carbyne Stack Ephemeral Service

[![codecov](https://codecov.io/gh/carbynestack/ephemeral/branch/master/graph/badge.svg?token=6QJMZ3MFUm)](https://codecov.io/gh/carbynestack/ephemeral)
[![Codacy Badge](https://app.codacy.com/project/badge/Grade/dde89cf6ed0d447292f2765c459b594c)](https://www.codacy.com?utm_source=github.com&utm_medium=referral&utm_content=carbynestack/ephemeral&utm_campaign=Badge_Grade)
[![Known Vulnerabilities](https://snyk.io/test/github/carbynestack/ephemeral/badge.svg)](https://snyk.io/test/github/carbynestack/ephemeral)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-%23FE5196?logo=conventionalcommits&logoColor=white)](https://conventionalcommits.org)
[![pre-commit](https://img.shields.io/badge/pre--commit-enabled-brightgreen?logo=pre-commit&logoColor=white)](https://github.com/pre-commit/pre-commit)
[![Contributor Covenant](https://img.shields.io/badge/Contributor%20Covenant-2.1-4baaaa.svg)](CODE_OF_CONDUCT.md)

Ephemeral is a serverless compute service for secure multiparty computation
based on [Knative](https://knative.dev), [Istio](https://istio.io) and
[Kubernetes](https://kubernetes.io).

> **DISCLAIMER**: Carbyne Stack Ephemeral is *alpha* software. The software is
> not ready for production use. It has neither been developed nor tested for a
> specific use case. The underlying Secure Multiparty Computation protocols are
> *currently* used in a way that is not secure.

Ephemeral is composed of these components:

- **[Ephemeral](cmd/ephemeral)** - The Knative user container that enables
  execution of MPC functions within a Carbyne Stack virtual cloud. Supports
  [MP-SPDZ](https://github.com/data61/MP-SPDZ) as the underlying MPC engine (see
  also
  [Carbyne Stack Ephemeral SPDZ Base Image](https://github.com/carbynestack/ephemeral-spdz-base-image)).

- **[Discovery Service](cmd/discovery)** - Coordinates the execution of
  functions across Carbyne Stack virtual cloud providers.

- **[Network Controller](cmd/network-controller)** - Enables communication
  between MPC engines by configuring Istio to route incoming traffic from remote
  MPC engines to the Ephemeral Knative pods.

- **[Client](ephemeral-java-client)** - A Java client that can be used to invoke
  Ephemeral functions.

- **[Helm Chart](charts/ephemeral)** - A Helm chart to deploy Ephemeral on a
  Kubernetes cluster.

## Namesake

> *Ephemeral* (plural ephemerals): Something which lasts for a short period of
> time.

As of [Wikipedia](https://en.wikipedia.org/wiki/Ephemerality):

> *Ephemerality* (from Greek *ephemeros*, literally "lasting only one day") is
> the concept of things being transitory, existing only briefly. Typically, the
> term ephemeral is used to describe objects found in nature, although it can
> describe a wide range of things, including human artifacts intentionally made
> to last for only a temporary period, in order to increase their perceived
> aesthetic value.

## Authoring Ephemeral Functions

Ephemeral uses the [MP-SPDZ](https://github.com/data61/MP-SPDZ) library as the
underlying MPC engine. That means you write Ephemeral functions using the Python
dialect used by MP-SPDZ.

### I/O

I/O is implemented in Ephemeral using socket functionality provided by MP-SPDZ.
A function for adding two secret shared values from and writing the result back
to [Amphora](https://github.com/carbynestack/amphora/) looks like this:

```python
# Open socket for I/O
listen(10000)
client_socket_id = regint()
acceptclientconnection(client_socket_id, 10000)

# Read all input data at once
v = sint.read_from_socket(client_socket_id, 2)
a = v[0]
b = v[1]

# Compute result
sum = a + b

# Pack result into array and write to socket
resp = Array(1, sint)
resp[0] = sum
sint.write_to_socket(client_socket_id, resp)
```

## Known issues

### Old Knative revisions must be deleted manually

Patching of Kubernetes Pods managed by Knative causes dangling old revisions
when a new revision is created. While the new revision is activated and traffic
is forwarded correctly, the old pods belonging to the previous revision are
lying around and must be manually deleted. The following commands must be used:

```bash
kubectl get revisions
# Pick up the older revision that must deleted, e.g. <REVISION_NAME>
# And delete it manually.
kubectl delete revision <REVISION_NAME>
```

## License

Carbyne Stack *Ephemeral* is open-sourced under the Apache License 2.0. See the
[LICENSE](LICENSE) file for details.

### 3rd Party Licenses

For information on how license obligations for 3rd party OSS dependencies are
fulfilled see the [README](https://github.com/carbynestack/carbynestack) file of
the Carbyne Stack repository.

## Contributing

Please see the Carbyne Stack
[Contributor's Guide](https://github.com/carbynestack/carbynestack/blob/master/CONTRIBUTING.md).
