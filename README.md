# eth-metrics

[![Tag](https://img.shields.io/github/tag/bilinearlabs/eth-metrics.svg)](https://github.com/bilinearlabs/eth-metrics/releases/)
[![Release](https://github.com/bilinearlabs/eth-metrics/actions/workflows/release.yml/badge.svg)](https://github.com/bilinearlabs/eth-metrics/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/bilinearlabs/eth-metrics)](https://goreportcard.com/report/github.com/bilinearlabs/eth-metrics)
[![Tests](https://github.com/bilinearlabs/eth-metrics/actions/workflows/tests.yml/badge.svg)](https://github.com/bilinearlabs/eth-metrics/actions/workflows/tests.yml)
[![gitpoap badge](https://public-api.gitpoap.io/v1/repo/bilinearlabs/eth-metrics/badge)](https://www.gitpoap.io/gh/bilinearlabs/eth-metrics)

## Introduction

Monitor the performance of your ethereum consensus staking pool. Just input the validator addresses you want to monitor. These are the parameters that are tracked:
* Rates of faulty head/source/target votes (see GASPER algorithm)
* Delta in rewards/penalties between consecutive epochs
* Proposed and missed blocks for each epoch

All data is stored in a SQLite database.

## Requirements

This project requires:
* An ethereum `consensus` client compliant with the http api running with `--enable-debug-rpc-endpoints`.
* An ethereum `execution` client compliant with the http api

## Build

### Docker

Note that the docker image is publicly available and can be fetched as follows:

```console
docker pull bilinearlabs/eth-metrics:latest
```

Build with docker:

```console
git clone https://github.com/bilinearlabs/eth-metrics.git
docker build -t eth-metrics .
```

### Source

```console
git clone https://github.com/bilinearlabs/eth-metrics.git
go build
```

## Usage

The following flags are available:

```console
./eth-metrics --help
```

You can monitor a set of `keys.txt` as follows.

Example `keys.txt` file:

```
0xaddc693f9090db30a9aae27c047a95245f60313f574fb32729dd06341db55c743e64ba0709ee74181750b6da5f234b44
0xb6ba7d587c26ca22fd9b306c2f6708c3d998269a81e09aa1298db37ed3ca0a355c46054cb3d3dfd220461465b1bdf267
```

You can pass as many `--pool-name` as you want. The name of the pool will be taken from the file, `keys` in this case.

```console
./eth-metrics \
--eth1address=https://your-execution-endpoint \
--eth2address=https://your-consensus-endpoint \
--verbosity=debug \
--database-path=db.db \
--pool-name=keys.txt
```

## Support

This project gratefully acknowledges the Ethereum Foundation for its support through their grant FY22-0795.
