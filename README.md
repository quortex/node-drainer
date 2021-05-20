# node-drainer
A kubernetes node draining scheduler.

## Overview
This project is a utility allowing to drain nodes within a Kubernetes cluster according to specific rules.

The main use case is the preventive drain of nodes on spot instances at desired times to minimize the risk of preemptions during peak usage.

## Installation

### Helm

Follow node-drainer documentation for Helm deployment [here](./helm/node-drainer).

## Configuration

### <a id="Configuration_Optional_args"></a>Optional args
The node-drainer container takes as argument the parameters below.
| Key                  | Description                                                                                    | Default |
| -------------------- | ---------------------------------------------------------------------------------------------- | ------- |
| l                    | Selector to list the nodes to drain on labels separated by commas (e.g. `-l foo=bar,bar=baz`). | `""`    |
| older-than           | The minimum lifespan that a node must have to be drained.                                      | `8h`    |
| count                | The number of nodes to drain.                                                                  | `1`     |
| max-unscheduled-pods | The maximum number of unscheduled pods on the cluster beyond which the drain will fail.        | `0`     |
| eviction-timeout     | The timeout in seconds for pods eviction during node drain.                                    | `300`   |
| dev                  | Enable dev mode for logging.                                                                   | `false` |
| v                    | Logs verbosity. 0 => panic, 1 => error, 2 => warning, 3 => info, 4 => debug                    | 3       |
| asg-poll-interval    | AutoScaling Groups polling interval (used to generate custom metrics about ASGs).              | 30      |


## Supervision

### Logs
By default, node-drainer produces structured logs, with "Info" verbosity. These settings can be configured as described [here](#Configuration_Optional_args).

## License
Distributed under the Apache 2.0 License. See `LICENSE` for more information.

## Versioning
We use [SemVer](http://semver.org/) for versioning.

## Help
Got a question?
File a GitHub [issue](https://github.com/quortex/node-drainer/issues).
