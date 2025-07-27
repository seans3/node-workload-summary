# Workload-Aware Node Summary

**Authors:** Eric Tune, Sean Sullivan  
**Date:** July 26, 2025  
**Status:** Draft

## 1. Motivation

In large-scale Kubernetes clusters, particularly those used for AI/ML, understanding the state of nodes and the distribution of workloads becomes challenging. Standard tools like `kubectl get nodes` are inefficient, slow, and produce verbose output that is difficult to parse across thousands of nodes. This leads to high operational costs and slow response times for administrators trying to diagnose issues or assess cluster utilization.

This project introduces a solution based on the "NodeSummary" concept, using Custom Resource Definitions (CRDs) to create efficient, pre-aggregated summaries of nodes and their associated workloads. The core goal is to implement a `kubectl get workloads` command that is scalable and extensible, making workload types and node groupings fully parameterizable.

## 2. Goals

- **Provide Fast Summaries:** Drastically reduce the time and data required to get a summary of node status in a large cluster.
- **Summarize Workloads:** Implement a mechanism to identify and summarize high-level workloads (e.g., JobSet, Deployment, LeaderWorkerSet) by aggregating their constituent pods.
- **Link Workloads to Nodes:** Show the relationship between workloads and the groups of nodes they are running on.
- **Parameterization:** Allow administrators to define which API types are considered "workloads" and which node labels should be used for grouping nodes.

## 3. High-Level Design

We will introduce two primary sets of CRDs and their corresponding controllers: one set for summarizing nodes (`NodeSummarizer`, `NodeSummary`) and another for summarizing workloads (`WorkloadSummarizer`, `WorkloadSummary`). The interaction between these controllers will provide the necessary data to link workloads to the node groups they occupy.

### CRD and Controller Architecture

#### 1. Node Summarization

A `NodeSummarizer` (CRD) is created by an administrator to specify how to group nodes. The `spec` contains a single field, `labelKey`, which defines the node label to use for grouping. The controller for `NodeSummarizer` will then create a `NodeSummary` object for each unique value of that label across all nodes.

**`NodeSummarizer` Example:**
```yaml
apiVersion: "ux.k8s.io/v1alpha1"
kind: NodeSummarizer
metadata:
  name: "nodepool-summarizer"
spec:
  # The node label key used to group nodes.
  labelKey: "cloud.google.com/gke-nodepool"
```

**`NodeSummary` Example:**
```yaml
apiVersion: "ux.k8s.io/v1alpha1"
kind: NodeSummary
metadata:
  # Name is derived, e.g., nodepool-summarizer-big-tpu-pool
  name: "nodepool-summarizer-big-tpu-pool"
  ownerReferences:
  - apiVersion: "ux.k8s.io/v1alpha1"
    kind: NodeSummarizer
    name: "nodepool-summarizer"
spec:
  # The selector is set by the controller and is immutable.
  selector:
    matchLabels:
      "cloud.google.com/gke-nodepool": "big-tpu-pool"
status:
  nodeCount: 512
  nodeNamesPrefix: "gke-tpu-20ee2cce-*"
  conditions:
  - type: Ready
    count: 510
  - type: NotReady
    count: 2
  allocatable:
    cpu: "5120"
    memory: "20480Gi"
    "nvidia.com/gpu": "512"
```

#### 2. Workload Summarization

A `WorkloadSummarizer` defines which root-level objects are considered "workloads". The controller for `WorkloadSummarizer` watches for pods and follows their `ownerReferences` to find the root workload object. It then creates or updates a `WorkloadSummary` for that workload.

**`WorkloadSummarizer` Example:**
```yaml
apiVersion: "ux.k8s.io/v1alpha1"
kind: WorkloadSummarizer
metadata:
  name: "ai-workloads"
spec:
  # Defines which root-level objects are considered workloads.
  workloadTypes:
  - group: "batch.x-k8s.io"
    kind: "JobSet"
  - group: "x-k8s.io"
    kind: "LeaderWorkerSet"
  - group: "apps"
    kind: "Deployment"
```

**`WorkloadSummary` Example:**
```yaml
apiVersion: "ux.k8s.io/v1alpha1"
kind: WorkloadSummary
metadata:
  # Name is derived from the workload instance, e.g., default-train
  name: "default-train"
  namespace: "default"
status:
  podCount: 8192
  shortType: "js" # From WorkloadSummarizer config
  longType: "jobset.batch.x-k8s.io/v1"
  # References to the node groups this workload is running on.
  nodeSummaryRefs:
  - name: "nodepool-summarizer-tpu-v5p-res-84395"
    nodeCount: 512 # Number of nodes in that group used by this workload.
```

## 5. kubectl Integration

This design enables a powerful and efficient CLI experience. The command `kubectl get workloads` can be implemented by simply listing the `WorkloadSummary` resources.

**Command:**
```bash
kubectl get workloads
```

**Example Output:**
```
NAME   N_PODS   SHORT_TYPE   LONG_TYPE
vllm        4   lws          leaderworkerset.x-k8s.io/v1
train    8192   js           jobset.xk8s.io/v1
```

## 6. Example Workflow

1.  **Admin sets up summarization:** The cluster administrator creates a `NodeSummarizer` to group nodes by a label and a `WorkloadSummarizer` to track specific workload types.
2.  **Controller Action (Nodes):** The `nodesummarizer-controller` discovers all nodes and creates `NodeSummary` objects for each group.
3.  **User submits workload:** A data scientist submits a `JobSet` named `train`. Kubernetes schedules its pods onto various nodes.
4.  **Controller Action (Workloads):** The `workloadsummarizer-controller` detects the new pods, follows their `ownerReferences` up to the `train` `JobSet`, and identifies which `NodeSummary` groups the pods belong to.
5.  **Summary Creation:** The controller creates a `WorkloadSummary` named `train`, populating its status with the pod count and references to the `NodeSummary` groups being used.
6.  **User checks status:** An administrator runs `kubectl get workloads` and gets an instant, aggregated view without querying thousands of individual pods or nodes.