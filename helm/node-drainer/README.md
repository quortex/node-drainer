# node-drainer

![Version: 0.1.0](https://img.shields.io/badge/Version-0.1.0-informational?style=flat-square) ![Type: application](https://img.shields.io/badge/Type-application-informational?style=flat-square) ![AppVersion: 0.1.0](https://img.shields.io/badge/AppVersion-0.1.0-informational?style=flat-square)

A cronjob that drain nodes periodically.

## Usage

This project is a utility allowing to schedule nodes drains within a Kubernetes cluster according to specific rules.

The main use case is the preventive drain of nodes on spot instances at desired times to minimize the risk of preemptions during peak usage.

## Installation

1. Add node-drainer helm repository

```sh
helm repo add node-drainer https://quortex.github.io/node-drainer
```

2. Deploy the appropriate release in desired namespace.

```sh
helm install node-drainer node-drainer/node-drainer -n <NAMESPACE>>
```

## Values

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| schedule | string | `"0 0 * * *"` | The schedule in Cron format, see https://en.wikipedia.org/wiki/Cron. |
| selector | object | `{}` | Selector to list the nodes to drain (matching labels). |
| olderThan | string | `"8h"` | The minimum lifespan that a node must have to be drained. |
| count | int | `1` | The number of nodes to drain. |
| maxUnscheduledPods | int | `0` | The maximum number of unscheduled pods on the cluster beyond which the drain will fail. |
| concurrencyPolicy | string | `"Allow"` | Specifies how to treat concurrent executions of a Job (Allow / Forbid / Replace). |
| backoffLimit | int | `6` | Specifies the number of retries before marking a job as failed. |
| successfulJobsHistoryLimit | int | `3` | The number of successful finished jobs to retain. |
| failedJobsHistoryLimit | int | `1` | The number of failed finished jobs to retain. |
| suspend | bool | `false` | This flag tells the controller to suspend subsequent executions, it does not apply to already started executions. |
| evictionTimeout | int | `300` | The timeout in seconds for pods eviction during node drain |
| logs.verbosity | int | `3` | Logs verbosity:  0 => panic  1 => error  2 => warning  3 => info  4 => debug |
| logs.enableDevLogs | bool | `false` |  |
| image.repository | string | `"eu.gcr.io/quortex-registry-public/node-drainer"` | node-drainer image repository. |
| image.tag | string | `"0.1.0"` | node-drainer image tag. |
| image.pullPolicy | string | `"IfNotPresent"` | node-drainer image pull policy. |
| rbac.create | bool | `true` |  |
| restartPolicy | string | `"OnFailure"` | node-drainer restartPolicy (supported values: "OnFailure", "Never"). |
| imagePullSecrets | list | `[]` | A list of secrets used to pull containers images. |
| nameOverride | string | `""` | Helm's name computing override. |
| fullnameOverride | string | `""` | Helm's fullname computing override. |
| resources | object | `{}` | node-drainer container required resources. |
| podAnnotations | object | `{}` | Annotations to be added to pods. |
| nodeSelector | object | `{}` | Node labels for node-drainer pod assignment. |
| tolerations | list | `[]` | Node tolerations for node-drainer scheduling to nodes with taints. |
| affinity | object | `{}` | Affinity for node-drainer pod assignment. |

