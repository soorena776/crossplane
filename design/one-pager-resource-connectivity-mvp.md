# Resource Connectivity

* Owner: Nic Cope (@negz)
* Reviewers: Crossplane Maintainers
* Status: Draft

## Terminology

* _External resource_. An actual resource that exists outside Kubernetes,
  typically in the cloud. AWS RDS and GCP Cloud Memorystore instaces are
  external resources.
* _Managed resource_. The Crossplane representation of an external resource.
  The `RDSInstance` and `CloudMemorystoreInstance` Kubernetes kinds are managed
  resources. A managed resource models the satisfaction of a need; i.e. the need
  for a Redis Cluster is satisfied by the allocation (aka binding) of a
  `CloudMemoryStoreInstance`.
* _Resource claim_. The Crossplane representation of a request for the
  allocation of a managed resource. Resource claims typically represent the need
  for a managed resource that implements a particular protocol. `MySQLInstance`
  and `RedisCluster` are examples of resource claims.
* _Resource class_. The Crossplane representation of the desired configuration
  of a managed resource. Resource claims reference a resource class in order to
  specify how they should be satisfied by a managed resource.
* _Connection secret_. A Kubernetes `Secret` encoding all data required to
  connect to (or consume) an external resource.
* _Claimant_ or _consumer_. The Kubernetes representation of a process wishing
  to connect to a managed resource, typically a `Pod` or some abstraction
  thereupon such as a `Deployment` or `KubernetesApplication`.

## Background

Crossplane models _external resources_, for example an infrastructure resource
in a cloud provider's API, as declarative Kubernetes resources. These
declarative _managed resources_ ensure their underlying external resources
reflect their owner's desired state. Application owners request access to (which
may imply the creation of) a managed resource by creating a _resource claim_.
Contemporary resource claims expose little to no configuration details to the
application owner, instead referencing a _resource class_ that specifies how the
underlying managed resource, and thus the external resource, should be
configured.

Frequently application owners will want to create multiple resource claims and
ensure they can communicate with each other. For example an application owner
may wish to:

* Create a `KubernetesCluster` resource claim.
* Create a `MySQLInstance` resource claim.
* Deploy an application to the `KubernetesCluster`.
* Have said application use a database of the `MySQLInstance`.

This requires the external resources underlying the two resource claims to be
configured such that they can communicate with each other at a network level.
This configuration process varies from cloud provider to cloud provider and from
external resource to external resource. Frequently the operator of the external
resources is required to ensure they're both in the same cloud region, and/or
the same VPC network, subnetwork, security group, etc in order to communicate.
Certain settings may also need to be enabled on one or both external resources.
This highlights two shortcomings in Crossplane:

* Managed resources are not consistently 'high fidelity', in that they don't
  expose all of the settings their underlying external resource's API exposes.
  Crossplane does not consistently expose the settings necessary to configure
  connectivity.
* External resources often depend on other external resources, particularly
  network constructs like VPC networks, in order to configure connectivity.
  Crossplane does not model these external resources as managed resources,
  requiring the cloud administrator to create and manage them outside of
  Crossplane.

This document will walk through an example scenario for each supported cloud
provider in which an application operator wishes to deploy [Wordpress] to a
`KubernetesCluster`, backed by a `MySQLInstance`, highlighting the minimum
changes necessary for Crossplane to support this.

### Low Fidelity Managed Resources

The below examples illustrate the fidelity of two Crossplane managed resources
in relation to their equivalent API objects. In each case a comprehensive YAML
document representing every supported field of the Crossplane managed resource
is compared to a hypothetical YAML document resulting from directly translating
the cloud provider API. Note that these examples are primarily intended to
illustrate the at times significant difference between the configuration a cloud
provider API supports, and the configuration Crossplane exposes. _Complete_
fidelity in Crossplane managed resources is not necessary to enable connectivity
between managed resources; it would be sufficient in the context of this design
to expose only any missing connectivity-related fields.

### CloudSQL

First, an exhaustive example of the settings Crossplane's `CloudSQLInstance`
currently supports. The `CloudSQLInstance` resource claim controller supports
reading all of the below settings from a `ResourceClass` at dynamic provisioning
time.

```yaml
---
apiVersion: database.gcp.crossplane.io/v1alpha1
kind: CloudSQLInstance
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  tier: db-n1-standard-1
  authorizedNetworks:
  - 73.98.0.0/16
  databaseVersion: MYSQL_5_6
  region: us-central1
  storageType: PD_SSD
  storageGB: 50
  labels:
    cool: very
```

By comparison, a direct translation of the [CloudSQL external resource]'s
writable API object fields to a Kubernetes YAML specification would be as
follows:

```yaml
---
apiVersion: database.gcp.crossplane.io/v1alpha1
kind: CloudSQLInstance
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  databaseVersion: MYSQL_5_6
  region: us-central1
  settings:
    authorizedGaeApplications:
    - mycoolapp
    tier: db-n1-standard-1
    backupConfiguration:
      startTime: 2019-01-01 00:00:00
      location: us-west1
      enabled: true
      binaryLogEnabled: true
      replicationLogArchivingEnabled: true
    pricingPlan: PER_USE
    replicationType: ASYNCHRONOUS
    activationPolicy: ON_DEMAND
    ipConfiguration:
      ipv4Enabled: true
      # authorizedNetworks whitelists the specified _public_ CIDRs.
      authorizedNetworks:
      - name: mycoolcidr
        value: 73.98.0.0/16
        expirationTime: datetime
      requireSsl: false
      # privateNetwork whitelists any instance in the specified GCP VPC network.
      privateNetwork: /projects/mycoolproject/global/networks/mycoolvpc
    locationPreference:
      followGaeApplication: mycoolapp
      zone: us-central1-a
    databaseFlags:
    - name: cool
      value: very
    databaseReplicationEnabled: true
    crashSafeReplicationEnabled: true
    dataDiskSizeGb: 50
    dataDiskType: PD_SSD
    maintenanceWindow:
      hour: 23
      day: 2
      updateTrack: stable
    storageAutoResize: true
    storageAutoResizeLimit: 500
    userLabels:
      cool: very
  masterInstanceName: mycoolmastername
  failoverReplica:
    name: mycoolreplicaname
  replicaConfiguration:
    mysqlReplicaConfiguration:
      dumpFilePath: dump
      username: cooluser
      password: secretpassword
      connectRetryInterval: 60
      masterHeartbeatPeriod: 100
      caCertificate: PEMPEMPEM
      clientCertificate: PEMPEMPEM
      clientKey: PEMPEMPEM
      sslCipher: supersecure
      verifyServerCertificate: true
    failoverTarget: false
```

#### Google Kubernetes Engine

The Crossplane `GKECluster` managed resource is particularly misleading. Its API
definition has a fairly comprehensive (albeit confusingly flattened) facsimile
of the associated GKE API object, but only a small subset of these fields are
actually parsed and submitted to the GKE API. Only the fields that are parsed
and submitted are included in the below example:

```yaml
---
apiVersion: compute.gcp.crossplane.io/v1alpha1
kind: GKECluster
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  clusterVersion: 1.13
  createSubnetwork: false
  enableIPAlias: true
  labels:
    cool: very
  machineType: n1-standard-2
  clusterIPV4CIDR: 192.168.0.0/16
  nodeIPV4CIDR: 10.0.0.0/8
  serviceIPV4CIDR: 172.16.0.0/24
  nodeLabels:
  - coollabel
  numNodes: 6
  zone: us-central1-a
  Scopes:
  - mycoolnodescope
  # This setting is not currently read from the GKE resource class.
  username: cooluser
```

By comparison, a direct translation of the [GKE cluster external resource]'s
writable API object fields to a Kubernetes YAML specification would be as
follows. Note that the GKE API contains several deprecated fields, all of which
are superceded by others (e.g. `nodeConfig` is superceded by `nodePools`). The
below translation omits these deprecated fields.

```yaml
---
apiVersion: compute.gcp.crossplane.io/v1alpha1
kind: GKECluster
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  description: My cool cluster
  masterAuth:
    username: cooluser
    password: secretpassword
    clientCertificateConfig:
      issueClientCertificate: true
  loggingService: logging.googleapis.com/kubernetes
  monitoringService: monitoring.googleapis.com
  network: mycoolnetwork
  clusterIpv4Cidr: 10.0.0.0/8
  addonsConfig:
    httpLoadBalancing:
      disabled: false
    horizontalPodAutoscaling:
      disabled: false
    networkPolicyConfig:
      disabled: false
  subnetwork: mycoolsubnetwork
  nodePools:
  - name: string
    config:
      machineType: n1-standard-2
      diskSizeGb: 20
      oauthScopes:
      - cool
      serviceAccount: mycoolserviceaccount@accounts.google.com
      metadata:
        cool: very
      imageType: coreos/stable
      labels:
        cool: true
      localSsdCount: 0
      tags:
      - cool
      preemptible: false
      accelerators:
        acceleratorCount: 2
        acceleratorType: large
      diskType: pd-ssd
      minCpuPlatform: Intel Haswell
      taints:
        - key: color
          value: purple
          effect: NO_SCHEDULE
    initialNodeCount: 3
    version: 1.14
    autoscaling:
      enabled: true
      minNodeCount: 3
      maxNodeCount: 9
    management:
      autoUpgrade: true
      autoRepair: true
    maxPodsConstraint:
      maxPodsPerNode: 254
  locations:
  - us-central1-a
  enableKubernetesAlpha: false
  resourceLabels:
    cool: very
  legacyAbac:
    enabled: false
  networkPolicy:
    provider: CALICO
    enabled: true
  ipAllocationPolicy:
    useIpAliases: true
    createSubnetwork: false
    subnetworkName: mycoolsubnetwork
    clusterSecondaryRangeName: mycoolpodsrange
    servicesSecondaryRangeName: mycoolservicesrange
    clusterIpv4CidrBlock: 10.0.0.0/8
    nodeIpv4CidrBlock: 172.16.1.0/24
    servicesIpv4CidrBlock: 192.168.0.0/24
    tpuIpv4CidrBlock: 172.16.2.0/24
  masterAuthorizedNetworksConfig:
    enabled: true
    cidrBlocks:
      displayName: coolcidr
      cidrBlock: 93.80.0.0/16
  maintenancePolicy: object (MaintenancePolicy)
    window:
      dailyMaintenanceWindow:
        startTime: 01:30
  defaultMaxPodsConstraint:
    maxPodsPerNode: 254
  resourceUsageExportConfig:
    bigqueryDestination:
      datasetId: mycooldataset
    enableNetworkEgressMetering: true
    consumptionMeteringConfig:
      enabled: true
  privateClusterConfig:
    enablePrivateNodes: true
    enablePrivateEndpoint: true
    masterIpv4CidrBlock: 172.16.0.0/16
  initialClusterVersion: 1.13
  enableTpu: false
```

## Proposal

Minimum viable support for resource connectivity can be enabled by ensuring
three things.

* All external resources required to configure connectivity have corresponding
  Crossplane managed resources, such that the cloud administrator does not need
  to leave Crossplane to configure them. For example, if a `KubernetesCluster`
  can only connect to a `MySQLInstance` in the same VPC network then Crossplane
  must be able to represent said VPC network as a managed resource.
* All external resource settings required to configure connectivity must be
  exposed in the configuration of their associated managed resource.
* All managed resource settings required to configure connectivity must be
  exposed in the configuration of their associated resource class.

With the above in place resource connectivity between a `KubernetesCluster` and
a `MySQLInstance` can be configured roughly as follows:

1. The cloud administrator creates any managed resources necessary to connect
   a Kubernetes cluster to a MySQL database in their cloud of choice, for
   example they create a VPC network for both external resources to live in.
1. The cloud administrator creates resource classes to be used when creating
   `KubernetesCluster` and `MySQLInstance` resource claims against their cloud
   of choice. These resource classes specify the necessary configuration to
   ensure any managed resources backing the aforementioned claims can connect,
   for example ensuring they're both configured to use the aforementioned VPC
   network.
1. The app operator creates their `KubernetesCluster` and `MySQLInstance`
   resource claims, which either explicitly reference or default to the resource
   classes created in step 2. Because their underlying managed resources are
   appropriately configured, they can now communicate!

The remainder of this proposal works through this scenario in each cloud
provider, highlighting the changes necessary to enable connectivity.

### Resource References

It's typical for Kubernetes resources to refer to each other. The [Kubernetes
API conventions] state:

> References to loosely coupled sets of objects, such as pods overseen by a
> replication controller, are usually best referred to using a label selector.
> [...] References to specific objects, especially specific resource versions
> and/or specific fields of those objects, are specified using the
> ObjectReference type (or other types representing strict subsets of it).
> [...] Object references should either be called fooName if referring to an
> object of kind Foo by just the name (within the current namespace, if a
> namespaced resource), or should be called fooRef, and should contain a subset
> of the fields of the ObjectReference type.

Crossplane currently uses these conventions to create references between
resource claims and classes, resource claims and managed resources, etc. No
pattern currently exists for modelling relationships between managed resources.

Assume a `Network` managed resource named `kubernetesname` exists in namespace
`crossplane-system`. Further assume this managed resource represents a GCP VPC
network whose 'real' name in the GCP API is `externalname`. Finally, assume a
`CloudSQLInstance` managed resource wants to specify this network as its
`.spec.ipConfiguration.privateNetwork` (i.e. whitelisted VPC network). This
could be implemented in one of two ways:

1. The `CloudSQLInstance` simply requires `.spec.ipConfiguration.privateNetwork`
   be provided as a `string` name specifying the real, external name of the
   `Network`, i.e. `externalname`. It is up to the creator of the
   `CloudSQLInstance` (or the `ResourceClass` used to create it) to ensure this
   name corresponds with the name of the `kubernetesname` `Network`.
1. The `CloudSQLInstance` requires an `ObjectReference` at be specified as
   `.spec.ipConfiguration.privateNetworkRef`. This `ObjectReference` refers to
   `kind: Network`, `namespace: crossplane-system`, `name: kubernetesname`. The
   `CloudSQLInstance` controller must then lookup the specified `Network` in
   order to determine that it is named `externalname` when submitting requests
   to the GCP API.

This document suggests the first approach, both because it requires less logic
and fewer Kubernetes API calls to implement, and because it enables Crossplane
users to reference both external resources that are and are not modelled as
Crossplane managed resources.

### Google Cloud Platform

In the Google Cloud Platform (GCP) `KubernetesCluster` and `MySQLInstance`
claims are satisfied by `GKECluster` and `CloudSQLInstance` managed resources
respectively. A pod in a GKE cluster can [connect to a database] of a CloudSQL
instance either via Google's network, or via the public internet using a proxy.
This document focuses on the former strategy:

* The CloudSQL instance, which exists in a Google-managed VPC network, must be
  configured to enable access from a specific VPC network managed by the
  infrastructure operator. This network must have at least one subnetwork in the
  same region as the CloudSQL instance.
* The GKE cluster must be configured to create its nodes in the VPC network to
  which the CloudSQL instance is attached. This can be done by specifying the
  VPC network name and allowing the GKE cluster to create its own subnetwork,
  or by specifying an existing subnetwork.
* The GKE cluster must be [VPC native], i.e. configured to be [Alias IP]
  enabled. This means its pods are allocated IP addresses from a secondary IP
  range from within the aforementioned subnetwork.

The _absolute minimum_ required to support private network connectivity between
a `GKECluster` and a `CloudSQLInstance` is to leverage the `default` VPC network
that is created automatically at GCP project creation time. i.e.:

* Use the `GKECluster` managed resource's existing support for setting the
  [GKE cluster external resource]'s `.ipAllocationPolicy.useIpAliases` and
  `.ipAllocationPolicy.createSubnetwork` fields.
* Leverage the [GKE cluster external resource]'s default behaviour of using the
  project's `default` VPC network if none is specified.
* Add support for `CloudSQLInstance` managed resources and their resource claims
  to specify the external resource's `.ipConfiguration.privateNetwork`, and
  specify the project's `default` VPC network when creating classes.

To add a little more flexibility Crossplane could:

* Add support for creating a new VPC network via a Crossplane VPC `Network`
  managed resource.
* Add support for `CloudSQLInstance` managed resources and their resource
  classes to specify the external resource's `.ipConfiguration.privateNetwork`,
  and specify the external name of a `Network` managed resource.
* Add support for `GKECluster` managed resources and their resource classes to
  specify the [GKE cluster external resource]'s `.network` field, and specify
  the external name of a `Network` managed resource.
* Use the `GKECluster` managed resource's existing support for setting the
  [GKE cluster external resource]'s `.ipAllocationPolicy.useIpAliases` and
  `.ipAllocationPolicy.createSubnetwork` fields to ensure an appropriate VPC
  native subnet is automatically created and used.

To add a generous amount of flexibility Crossplane could:

* Add support for creating a new VPC network via a Crossplane VPC `Network`
  managed resource.
* Add support for creating a new VPC subnetwork via a VPC `Subnetwork` managed
  resource.
* Add support for `CloudSQLInstance` managed resources and their resource
  classes to specify the external resource's `.ipConfiguration.privateNetwork`,
  and specify the external name of a `Network` managed resource.
* Add support for `GKECluster` managed resources to specify the [GKE cluster
  external resource]'s `.network` and `.subnetwork` fields.
* Add support for `GKECluster` managed resources and their resource classes to
  specify the [GKE cluster external resource]'s `.network` field and
  `.subnetwork` fields, and specify the external names of a `Network` and a
  `Subnetwork` managed resource.
* Use the `GKECluster` managed resource's existing support for setting the
  [GKE cluster external resource]'s `.ipAllocationPolicy.useIpAliases` field.

High-fidelity Crossplane managed resource representations of the aforementioned
network and subnetwork external resources would look as follows:

```yaml
---
apiVersion: vpc.gcp.crossplane.io/v1alpha1
kind: Network
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  description: A really cool VPC network
  # IPv4Range puts the network in subnetwork-less 'legacy mode'
  IPv4Range: 10.0.0.0/8
  # The documentation says this puts the VPC network in auto-create subnet mode.
  # In my experiments it seemed to create the VPC in legacy mode instead.
  autoCreateSubnetworks: true
  peerings:
  - name: peerwithsomeothercoolnetwork
    network: someothercoolnetwork
    autoCreateRoutes: true
    exchangeSubnetRoutes: true
  routingConfig:
    routingMode: REGIONAL
```

```yaml
---
apiVersion: vpc.gcp.crossplane.io/v1alpha1
kind: Subnetwork
metadata:
  namespace: crossplane-system
  name: example
spec:
  nameFormat: mycoolname
  description: My cool VPC subnetwork
  network: projects/coolproject/global/networks/coolestvpc
  ipCidrRange: 192.168.0.0/24
  region: us-central-1
  privateIpGoogleAccess: true
  secondaryIpRanges:
  - rangeName: pods
    ipCidrRange: 10.0.0.0/8
  - rangeName: services
    ipCidrRange: 172.16.0.0/16
  enableFlowLogs: true
  logConfig:
    enable: true
    flowSampling: 0.5
    metadata: INCLUDE_ALL_METADATA
    aggregationInterval: 5-min
```

Putting this all together, the infrastructure administrator would configure the
following to ensure that when an app operator created a `MySQLInstance` claim
and `KubernetesCluster` claim the two would have connectivity:

```yaml
---
# A Network managed resource.
apiVersion: vpc.gcp.crossplane.io/v1alpha1
kind: Network
metadata:
  namespace: crossplane-system
  name: example
spec:
  providerRef:
    namespace: crossplane-system
    name: example
  nameFormat: mycoolnetwork
  autoCreateSubnetworks: false
---
# A Subnetwork managed resource.
apiVersion: vpc.gcp.crossplane.io/v1alpha1
kind: Subnetwork
metadata:
  namespace: crossplane-system
  name: example
spec:
  providerRef:
    namespace: crossplane-system
    name: example
  nameFormat: mycoolsubnetwork
  # Create this subnet in the Network we created previously.
  # mycoolproject must match the Crossplane GCP Provider project.
  # mycoolnetwork must match the above Network managed resource's name.
  network: projects/mycoolproject/global/networks/mycoolnetwork
  ipCidrRange: 172.16.10.0/24
  region: us-central1
  privateIpGoogleAccess: true
  secondaryIpRanges:
  - rangeName: pods
    ipCidrRange: 10.0.0.0/8
  - rangeName: services
    ipCidrRange: 172.16.20.0/24
---
# A ResourceClass that sastisfies MySQLInstance claims using CloudSQLInstance
# managed resources.
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  namespace: crossplane-system
  name: default-mysqlinstance
  labels:
    # This resource class is the default for MySQLInstance resource claims.
    mysqlinstance.database.crossplane.io/default: "true"
providerRef:
  namespace: crossplane-system
  name: example
parameters:
  nameFormat: mycoolname
  databaseVersion: MYSQL_5_6
  region: us-central1
  tier: db-n1-standard-1
  dataDiskSizeGb: "50"
  dataDiskType: PD_SSD
  # Allow access to this CloudSQL instance from the Network we created previously.
  # mycoolproject must match the Crossplane GCP Provider project.
  # mycoolnetwork must match the above Network managed resource's name.
  privateNetwork: /projects/mycoolproject/global/networks/mycoolnetwork
---
# A ResourceClass that sastisfies KubernetesCluster claims using GKECluster
# managed resources.
apiVersion: core.crossplane.io/v1alpha1
kind: ResourceClass
metadata:
  namespace: crossplane-system
  name: default-kubernetescluster
  labels:
    # This resource class is the default for KubernetesCluster resource claims.
    kubernetescluster.compute.crossplane.io/default: "true"
providerRef:
  namespace: crossplane-system
  name: example
parameters:
  clusterVersion: "1.12"
  machineType: n1-standard-2
  numNodes: "3"
  zone: us-central1-a
  # Create nodes in the mycoolsubnetwork subnetwork of the mycoolnetwork network.5-min
  # mycoolnetwork must match the above Network managed resource's name.
  # mycoolsubnetwork must match the above Subnetwork managed resource's name.
  network: mycoolnetwork
  subnetwork: mycoolsubnetwork
  # Enable VPC native subnetworks.
  enableIPAlias: "true"
  # These must match the names of the secondary ranges configured in the above
  # Subnetwork managed resource. Multiple GKE clusters cannot share secondary
  # ranges, so this resource class can be used by exactly one KubernetesCluster
  # claim, which is not ideal.
  clusterSecondaryRangeName: pods
  servicesSecondaryRangeName: services
```

### Amazon Web Services

TBD.

### Microsoft Azure

TBD.

[Wordpress]: https://wordpress.com/
[connect to a database]: https://cloud.google.com/sql/docs/postgres/connect-kubernetes-engine
[VPC native]: https://cloud.google.com/kubernetes-engine/docs/how-to/alias-ips
[Alias IP]: https://cloud.google.com/vpc/docs/alias-ip
[CloudSQL external resource]: https://cloud.google.com/sql/docs/postgres/admin-api/v1beta4/instances#resource
[GKE cluster external resource]: https://cloud.google.com/kubernetes-engine/docs/reference/rest/v1/projects.locations.clusters#Cluster
[Kubernetes API conventions]: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
