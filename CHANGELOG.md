# Changelog

## [0.3.0](https://github.com/carbynestack/ephemeral/compare/service-v0.2.0...service-v0.3.0) (2025-03-04)


### ⚠ BREAKING CHANGES

* Secure Communication Channels via TLS ([#75](https://github.com/carbynestack/ephemeral/issues/75))

### Features

* Secure Communication Channels via TLS ([#75](https://github.com/carbynestack/ephemeral/issues/75)) ([4eb64f7](https://github.com/carbynestack/ephemeral/commit/4eb64f7a7f37e013079eb641660b4288bc5a7c3f))

## [0.2.0](https://github.com/carbynestack/ephemeral/compare/service-v0.1.13...service-v0.2.0) (2024-12-17)


### ⚠ BREAKING CHANGES

* **chart/service:** Add authorization ([#80](https://github.com/carbynestack/ephemeral/issues/80))

### Features

* **chart/service:** Add authorization ([#80](https://github.com/carbynestack/ephemeral/issues/80)) ([d2b2899](https://github.com/carbynestack/ephemeral/commit/d2b28994e1c08f01cbc2b510026d3c3918828354))
* **java-client:** upgrade dependencies ([#77](https://github.com/carbynestack/ephemeral/issues/77)) ([4f1d0e8](https://github.com/carbynestack/ephemeral/commit/4f1d0e81ccb11b73cb7d03ccc0f598e58f89678e))

## [0.1.13](https://github.com/carbynestack/ephemeral/compare/service-v0.1.12...service-v0.1.13) (2023-08-04)


### Bug Fixes

* **service/chart:** fix forced aborts of long-running computations ([#36](https://github.com/carbynestack/ephemeral/issues/36)) ([a131ae7](https://github.com/carbynestack/ephemeral/commit/a131ae74bd9d98d345d78d84a67388cbd783b36d))

## [0.1.12](https://github.com/carbynestack/ephemeral/compare/service-v0.1.11...service-v0.1.12) (2023-07-28)


### Bug Fixes

* **service:** extract the label part from what is returned by the metadata action ([#71](https://github.com/carbynestack/ephemeral/issues/71)) ([2243f0f](https://github.com/carbynestack/ephemeral/commit/2243f0f113e96925d1ca4e1f3c92bb51914c3e4c))

## [0.1.11](https://github.com/carbynestack/ephemeral/compare/service-v0.1.10...service-v0.1.11) (2023-07-27)


### Bug Fixes

* **service:** quote tags and labels argument to make ko working ([#69](https://github.com/carbynestack/ephemeral/issues/69)) ([973f067](https://github.com/carbynestack/ephemeral/commit/973f0673003530aae9a693f0799a6008850b269a))

## [0.1.10](https://github.com/carbynestack/ephemeral/compare/service-v0.1.9...service-v0.1.10) (2023-07-27)


### Bug Fixes

* **service:** quote labels given to ko ([#67](https://github.com/carbynestack/ephemeral/issues/67)) ([d3bdb0b](https://github.com/carbynestack/ephemeral/commit/d3bdb0b73b390a299b382cc7b2ed0d51ed0c27f4))

## [0.1.9](https://github.com/carbynestack/ephemeral/compare/service-v0.1.8...service-v0.1.9) (2023-07-27)


### Bug Fixes

* **service:** generate both tags and lables for ko correctly ([#65](https://github.com/carbynestack/ephemeral/issues/65)) ([1ccac01](https://github.com/carbynestack/ephemeral/commit/1ccac012cff5c4ce7bcf4de854515116a872b51c))

## [0.1.8](https://github.com/carbynestack/ephemeral/compare/service-v0.1.7...service-v0.1.8) (2023-07-27)


### Bug Fixes

* **service:** test without labels ([#63](https://github.com/carbynestack/ephemeral/issues/63)) ([5e1f1fa](https://github.com/carbynestack/ephemeral/commit/5e1f1fa14a7677e93253264bc3ba31a4c569227b))

## [0.1.7](https://github.com/carbynestack/ephemeral/compare/service-v0.1.6...service-v0.1.7) (2023-07-27)


### Bug Fixes

* **service:** provide labels to ko command as separate flags ([#61](https://github.com/carbynestack/ephemeral/issues/61)) ([74b9a9f](https://github.com/carbynestack/ephemeral/commit/74b9a9f7cefa06b52004a0001ab25ecc1acdd803))

## [0.1.6](https://github.com/carbynestack/ephemeral/compare/service-v0.1.5...service-v0.1.6) (2023-07-27)


### Bug Fixes

* **service:** add missing '=' when invoking ko ([#59](https://github.com/carbynestack/ephemeral/issues/59)) ([ebe4c84](https://github.com/carbynestack/ephemeral/commit/ebe4c8472a3883917dbc562cc7e571141ca55bd2))

## [0.1.5](https://github.com/carbynestack/ephemeral/compare/service-v0.1.4...service-v0.1.5) (2023-07-27)


### Bug Fixes

* **service:** fix publish workflow for service ([#57](https://github.com/carbynestack/ephemeral/issues/57)) ([570b6b6](https://github.com/carbynestack/ephemeral/commit/570b6b6a701687aa0d76adfe8b54540f16884cb1))

## [0.1.4](https://github.com/carbynestack/ephemeral/compare/service-v0.1.3...service-v0.1.4) (2023-07-27)


### Bug Fixes

* **service:** update publish workflow to use correct working directory ([#55](https://github.com/carbynestack/ephemeral/issues/55)) ([0727a99](https://github.com/carbynestack/ephemeral/commit/0727a99081ee96f49b42fa45e72a30f0a6147548))

## [0.1.3](https://github.com/carbynestack/ephemeral/compare/service-v0.1.2...service-v0.1.3) (2023-07-27)


### Bug Fixes

* **service/java-client:** add support for partial builds using codeco… ([#52](https://github.com/carbynestack/ephemeral/issues/52)) ([5a7a591](https://github.com/carbynestack/ephemeral/commit/5a7a591c5d81daf8bad09826b9c9f8bfcbe73eee))

## [0.1.2](https://github.com/carbynestack/ephemeral/compare/service-v0.1.1...service-v0.1.2) (2023-07-26)


### Bug Fixes

* **service/chart/java-client:** fix filter logic for conditional triggering of builds ([#50](https://github.com/carbynestack/ephemeral/issues/50)) ([9f4f300](https://github.com/carbynestack/ephemeral/commit/9f4f30057e704470eff817b5ce4ae84890977b65))
* **service/chart/java-client:** test release logic with some minor fixes ([#46](https://github.com/carbynestack/ephemeral/issues/46)) ([a215b6b](https://github.com/carbynestack/ephemeral/commit/a215b6b884ea73fc69f4283aca849dbc8bf520d4))

## [0.1.1](https://github.com/carbynestack/ephemeral/compare/service-v0.1.0...service-v0.1.1) (2023-07-26)


### Bug Fixes

* **service/chart/java-client:** test release logic ([#39](https://github.com/carbynestack/ephemeral/issues/39)) ([9c8f07b](https://github.com/carbynestack/ephemeral/commit/9c8f07b53f7f9792ad2b484b25666c1a4244303d))
