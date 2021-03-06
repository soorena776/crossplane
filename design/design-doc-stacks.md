# Crossplane Stacks
* Owner: Jared Watts (@jbw976)
* Reviewers: Crossplane Maintainers
* Status: Accepted, revision 1.1

This document aims to provide details about the experience and implementation for Crossplane Stacks, which can add new functionality/support, types, and controllers to Crossplane.

## Revisions

* 1.1
  * Renamed Extensions concept to Stacks (`Stack` code references are unaffected) [#571](https://github.com/crossplaneio/crossplane/issues/571)
  * Added additional Questions and Open Issues

## Experience

The core experience for consuming new functionality in Crossplane is composed of 2 steps:

1. Create an installation request for the name of the Crossplane Stack or one of the CRDs that it owns
    1. e.g., GitLab or `gitlabcluster.gitlab.com/v1alpha1`
1. Create a CRD instance that the custom controller owns
    1. e.g., GitLab CRD instance

After step 1, the required types and controllers are available in the Crossplane cluster and this step only needs to be done once.

After step 2, the controller (or other supported “runner”) from the package performs the necessary operations to create workloads, claims, etc. that bring the users desired state captured in the CRD to reality.
Step 2 can be repeated many times to provision multiple "instances" of the types that the package introduces to the cluster.

## Terminology

* **Custom Resource Definition** - A standard Kubernetes CRD, which defines a new type of resource that can be managed declaratively. This serves as the unit of management in Crossplane.  The CRD is composed of spec and status sections and supports API level versioning (e.g., v1alpha1)
  * Atomic / External CRDs - usually represent external resources, and cannot be broken down any further (leaves)
  * Composite CRDs - these are also resources that capture parent/child relationships. They have a selector that can help query/find children resources.
  * Claim CRDs - these are abstract resources that bind to managed resources.
* **Custom Controllers** -- this is the implementation of one or more CRDs. Can be implemented in different ways, such as golang code (controller-runtime), templates, functions/hooks, templates, a new DSL, etc. The implementation itself is versioned using semantic versioning (e.g., v1.0.4)
* **Stack** -- this is the unit of extending Crossplane with new functionality.  It is comprised of the CRDs, Custom Controller, and metadata about the Stack.  A Crossplane cluster can be queried for all installed Stacks as a way of learning what functionality a particular Crossplane supports.
* **Stack Registry** -- this is a registry for Stacks where they can be published, downloaded, explored, categorized, etc. The registry understands a Stack’s custom controller and its CRDs and indexes by both -- you could lookup a custom controller by the CRD name and vice versa.
* **Stack Package** -- this is a package format for Stacks that contains the Stack definition, metadata, icons, CRDs, and other Stack specific files.  See [Stack Package Format](#stack-package-format).
* **Stack Manager (SM)** -- this is the component that is responsible for installing a Stack’s custom controllers and resources in Crossplane. It can download packages, resolve dependencies, install resources and execute controllers.  This component is also responsible for managing the complete life-cycle of Stacks, including upgrading them as new versions become available.
* **unpacking** -- the process of extracting the Stack files from a Stack Package.

These concepts comprise the extensibility story for Crossplane.  With them, users will be able to add new supported functionality of all varieties to Crossplane.
The currently supported functionality, such as PostgreSQL, Redis, etc. can be packaged and published using the concepts described above, so that the initial installation of Crossplane is very sparse.
Only the user’s desired functionality needs to be added on as needed basis (a la carte).

When Crossplane is initially created, we should consider only having a few key components installed and running:

* Core API types (CRDs)
* Scheduler
* Workload and Kubernetes cluster Controllers
* Stack Manager (SM)

This would enable a user to create Kubernetes clusters and define workloads to be scheduled on them without having to install any Stacks.
All further functionality for Crossplane (databases, buckets, etc.) could then be added through additional Stacks custom controllers and resources that are installed and managed by the SM.

## Installation Flow

This section describes the end to end installation flow implemented by the Stack Manager:

* The SM starts up with a default “source” registry (e.g. `registry.crossplane.io`) that contains packages (bundles of a Stack and its custom controllers and CRDs) published to it
* User creates a `StackRequest` instance to request a Stack be installed in the cluster, which includes everything needed to successfully run that Stack.  The `StackRequest` includes:
  * an optional source registry that can be any arbitrary registry location.  If this field is not specified then the SM's default source registry will be used.
  * One of the following must be specified:
    * package name (`gitlab`) OR
    * CRD name (`gitlabcluster.gitlab.com/v1alpha1`)
      * Note: this code path is exactly the same as dependency resolution
* The SM performs dependency resolution that determines all packages/Stacks that are required by this Stack and all of its dependent Stacks (Not Implemented)
* The SM pulls all necessary Stack packages from the registry
* The SM creates an unpack job that sends the artifacts to `stdout` which the SM ingests to install the Stack
  * Stack metadata (`app.yaml`, `install.yaml`, `rbac.yaml`) is extracted and transformed to create an `Stack` CRD instance that serves as a record of the install
  * All owned/defined CRDs are installed and annotated with their related metadata (`group.yaml`, `resource.yaml`, and icon file)
  * RBAC rules necessary for the controller or controller installer are installed (`rbac.yaml`)
  * Stack installation instructions (`install.yaml`), in the form of Kubernetes YAML state files, are parsed and sent to the Kubernetes API
* Kubernetes starts up the custom controller so that it is in the running state
* The SM marks the `StackRequest` status as succeeded

## `StackRequest` CRD

To commence the installation of new functionality into a Crossplane cluster, an instance of the `StackRequest` CRD should be created.
The SM will be watching for events on this type and it will begin the process of installing a Stack during its reconcile loop.

`StackRequests` can be specified by either a package name or by a CRD type.
When given a CRD type, the controller will query the registry to find out what package owns that CRD and then it will download that package to proceed with the install.
This gives more flexibility to how Stacks are installed and does not require the requestor to know what package a CRD is defined in.

```yaml
# request to extend Crossplane with the redis package,
# using a specific version number
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackRequest
metadata:
  name: redis-from-package
spec:
  source: registry.crossplane.io
  package: redis:v0.1.0
status:
  conditions:
  - type: Ready
    status: "True"
---
# request to extend Crossplane with the package that defines/owns,
# the rediscluster CRD
apiVersion: stacks.crossplane.io/v1alpha1
kind: StackRequest
metadata:
  name: redis-from-crd
spec:
  source: registry.crossplane.io
  crd: redisclusters.cache.crossplane.io/v1alpha1
status:
  conditions:
  - type: Creating
    status: "True"
```

## `Stack` CRD

The `Stack` CRD serves as a record of an installed Stack (a custom controller and its CRDs).
These records make it so that a user or system can query Crossplane and learn all of the functionality that has been installed on it as well as their statuses.

Instances of this CRD can be generated from the filesystem based contents of a package, i.e. the metadata files contained inside the package.
This can be thought of as a translation operation, where the file based content is translated into a YAML based version that is stored in the `Stack` CRD.

`Stack` CRD instances can also be created directly by a user without any knowledge of packages at all.
They can directly create any CRDs that their Stack requires and then create a `Stack` CRD instance that describes their Stack, its custom controller, etc.
The Stack Manager will see this new instance and take the steps necessary to ensure the custom controller is running in the cluster and the Stack’s functionality is available.

```yaml
apiVersion: stacks.crossplane.io/v1alpha1
kind: Stack
metadata:
 name: redis
spec:
 # these are references to CRDs for the resources exposed by this stack
 # by convention they are bundled in the same Package as this stack
 customresourcedefinitions:
  owns:
  - kind: RedisCluster
    apiVersion: crossplane.redislabs.com/v1alpha1
  dependsOn: []
  # CRDs that this stack depends on (required) are listed here
  # this data drives the dependency resolution process
 title: Redis stack for Crossplane
 description: "Markdown syntax about how Redis is a really cool database"
 version: 0.1.0
 icons:
 - base64data: iVBORw0KGgoAAAANSUhEUgAAAOEAAADZCAYAAADWmle6AAAACXBIWXMA
   mediatype: image/png
 maintainers:
 - name: Rick Kane
   email: rick@foo.io
 owners:
 - name: Chandler
   email: chandler@bar.io
 keywords:
 - "databases"
 links:
 - description: About
   url: "https://redislabs.com/"
 # the implementation of the stack, i.e. a controller that will run
 # on the crossplane cluster
 controller:
  deployment:
    name: redis-controller
    spec:
      replicas: 1
      selector:
        matchLabels:
          core.crossplane.io/name: "redis"
      template:
        metadata:
          name: redis-controller
          labels:
            core.crossplane.io/name: "redis"
        spec:
          serviceAccountName: redis-controller
          containers:
          - name: redis-controller
            image: redis/redis-crossplane-controller:2.0.9
            imagePullPolicy: Always
            env:
 # the permissions needed by the controller
 permissions:
   rules:
   - apiGroups:
     - ""
     resources:
     - secrets
     - serviceaccounts
     - events
     - namespaces
     verbs:
     - get
     - list
     - watch
     - create
     - update
     - patch
     - delete
```

## Stack Package Format

A Stack package is the bundle that contains the custom controller definition, CRDs, icons, and other metadata for a given Stack.

The Stack Package Format is essentially just a tarball (e.g., a [container image](https://github.com/opencontainers/image-spec/blob/master/spec.md)).  All of the Stack resources are brought together into this single unit which is understood and supported by the Stack registry and Stack manager.

As previously mentioned, after downloading and unpacking a Stack package, the Stack Manager will not only install its contents into Crossplane, but it will also translate them into a `Stack` record.

More details will be provided when a Stack registry project is bootstrapped and launched.

### Stack Filesystem Layout

Inside of a package, the filesystem layout shown below is expected for the best experience.  This layout will accommodate current and future Stack consuming tools, such as client tools, back-end indexing and cataloging tools, and the Stack Manager itself.

```text
.registry/
├── icon.svg
├── app.yaml # Application metadata.
├── install.yaml # Optional install metadata.
├── rbac.yaml # Optional RBAC permissions.
├── ui-schema.yaml #  Optional UI Metadata
└── resources
      └── databases.foocompany.io # Group directory
            ├── group.yaml # Optional Group metadata
            ├── icon.svg # Optional Group icon
            └── mysql # Kind directory by convention
                ├── v1alpha1
                │   ├── mysql.v1alpha1.crd.yaml # Required CRD
                │   ├── icon.svg # Optional resource icon
                │   └── resource.yaml # Resource level metadata.
                └── v1beta1
                    ├── mysql.v1beta1.crd.yaml
                    ├── ui-schema.yaml #  Optional UI Metadata
                    ├── icon.svg
                    └── resource.yaml
```

In this example, the directory names "databases.foocompany.io", "mysql", "v1alpha1", and "v1alpha2" are for human-readability and should be considered arbitrary.  The group, kind, and version data will be parsed from any leaf level CRD files ignoring the directory names.

### Stack Files

* `app.yaml`: This file is the general metadata and information about the Stack, such as its name, description, version, owners, etc.  This metadata will be saved in the `Stack` record's spec fields.
* `install.yaml`: This file contains the information for how the custom controller for the Stack should be installed into Crossplane.  Initially, only simple `Deployment` based controllers will be supported, but eventually other types of implementations will be supported as well, e.g., templates, functions/hooks, templates, a new DSL, etc.
* `icon.svg`: This file (or `icon.png`, `icon.jpg`, `icon.gif`, or potentially other supported filenames, TBD) will be used in a visual context when listing or describing this stack.  The preferred formats/filenames are `svg`, `png`, `jpg`, `gif` in that order (if multiple files exist).  For bitmap formats, the width to height ratio should be 1:1. Limitations may be placed on the acceptable file dimensions and byte size (TBD).
* `resources` directory: This directory contains all the CRDs and optional metadata about them.
  * `group.yaml`: Related Stack resources (CRDs) can be described at a high level within a group directory using this file.
  * `resource.yaml` and `icon.svg`: Files that describe the resource with descriptions, titles, or images, may be used to inform out-of-cluster or in-cluster Stack managing tools.  This may affect the annotations of the `Extension` record or the Resource CRD (TBD).
  * `ui-schema.yaml`: UI metadata that will be transformed and annotated according to the [Stack UI Metadata One Pager](one-pager-stack-ui-metadata.md)
  * `*crd.yaml`: These CRDs are the types that the custom controller implements the logic for.  They will be directly installed into Crossplane so that users can create instances of them to start consuming their new Stack functionality.  Notice that the filenames can vary from `very.descriptive.name.crd.yaml` to `crd.yaml`.
  Multiple CRDs can reside in the same file.  These CRDs may also be pre-annotated at build time with annotations describing the `resource.yaml`, `icon.svg`, and `ui-schema.yaml` files to avoid bundling additional files and incurring a minor processing penalty at runtime.

Examples of annotations that the Stack manager produces are included in the [Example Package Files](example-package-files) section.  Icon annotations should be provided as [RFC-2397](https://tools.ietf.org/html/rfc2397) Data URIs and there is a strong preference that these URIs use base64 encoding.

The minimum required file tree for a single tool, such as the Stack Manager, could be condensed to the following:

```text
.registry/
├── app.yaml
├── install.yaml
├── rbac.yaml
└── resources
      └── crd.yaml
```

Strictly speaking, `install.yaml` and `rbac.yaml` are optional, but a Stack bereft of these files would only introduce a data storage CRD with no active controller to act on CRs.  A Stack with no implementation could still be useful as a dependency of another Stack if CRs or the CRD itself can influence the behavior of active Stacks.

## Example Package Files

A concrete examples of this package format can be examined at <https://github.com/crossplaneio/sample-extension>.

A Git repository may choose to include the `.registry` directory with all of the files described above but that may not always be the case.  Stacks are easy to create as build artifacts through a combination of shell scripting, Make, Docker, or any other tool-chain or process that can produce an OCI image.

An example project that processes the artifacts of Kubebuilder 2 to create a Stack is available at <https://github.com/crossplaneio/sample-stack-wordpress>.

### Example `app.yaml`

```yaml
# Human readable title of application.
title: Sample Crossplane Stack

# Markdown description of this entry
description: |
 Markdown describing this sample Crossplane Stack project.

# Version of project (optional)
# If omitted the version will be filled with the docker tag
# If set it must match the docker tag
version: 0.0.1

# Maintainer names and emails.
maintainers:
- name: Jared Watts
  email: jared@upbound.io

# Owner names and emails.
owners:
- name: Bassam Tabbara
  email: bassam@upbound.io

# Human readable company name.
company: Upbound

# A single category that best fits this Stack
# Arbitrary categories may be used but it is expected that a preferred set of categories will emerge among Stack tools
category: Database

# Keywords that describe this application and help search indexing
keywords:
- "samples"
- "examples"
- "tutorials"

# Links to more information about the application (about page, source code, etc.)
website: "https://crossplane.io"
source: "https://github.com/crossplaneio/sample-stack"

# License SPDX name: https://spdx.org/licenses/
license: Apache-2.0
```

### Example `install.yaml`

The `install.yaml` file is expected to conform to a standard Kubernetes YAML file describing a single `Deployment` object.

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: crossplane-sample-extension
  labels:
    core.crossplane.io/name: "crossplane-sample-extension"
spec:
  selector:
    matchLabels:
      core.crossplane.io/name: "crossplane-sample-extension"
  replicas: 1
  template:
    metadata:
      name: sample-extension-controller
      labels:
        core.crossplane.io/name: "crossplane-sample-extension"
    spec:
      containers:
      - name: sample-extension-controller
        image: crossplane/sample-extension:latest
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

### Example `group.yaml`

```yaml
group: databases.foocompany.io
title: Sample Controller Types
description: |
    This is a crd group description
```

### Example `resource.yaml`

```yaml
resource: mysql
title: MySQL
category: Database
description: |
  # MySQL Resource by FooCompany

  This is the Crossplane Stack for FooCompany MySQL

  ## Details

  More markdown.
```

### Example `rbac.yaml`

```yaml
rules:
- apiGroups:
  - ""
  resources:
  - secrets
  - serviceaccounts
  - events
  - namespaces
  verbs:
  - get
  - list
  - watch
  - create
  - update
  - patch
  - delete
- apiGroups:
  - samples.crossplane.io
  resources:
  - "*"
  verbs:
  - "*"
```

### Example `crd.yaml`

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  labels:
    controller-tools.k8s.io: "1.0"
  name: mytypes.samples.crossplane.io
spec:
  group: samples.crossplane.io
  names:
    kind: Mytype
    plural: mytypes
  scope: Namespaced
  version: v1alpha1
```

#### Example `crd.yaml` with Stack annotations

It is the job of the SM or a Stack build tool to process the recommended meta-data files into the final CRD installed in the cluster.  These annotations will assist Stack tools in discovery and identification of resources in cluster that can be managed.

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: mytypes.samples.crossplane.io
  labels:
    controller-tools.k8s.io: "1.0"
  annotations:
    stacks.crossplane.io/name: "crossplane-sample-extension"
    stacks.crossplane.io/resource-group-title: "Title of the Group"
    stacks.crossplane.io/resource-group-description: |
        Description of the Group
    stacks.crossplane.io/resource-category: "Databases"
    stacks.crossplane.io/resource-title: "Title of the Resource"
    stacks.crossplane.io/resource-description: |
        Description of the Resource
    stacks.crossplane.io/icon-data-uri: data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciLz4K
    stacks.crossplane.io/ui-spec: |
      uiSpecVersion: 0.3
      uiSpec:
      - title: Configuration
        description: Enter information specific to the configuration you wish to create.
        items:
        - name: dbReplicas
          controlType: singleInput
          type: integer
          path: .spec.dbReplicas
          title: DB Replicas
          description: The number of DB Replicas
          default: 1
          validation:
          - minimum: 1
          - maximum: 3
      ---
      additionalSpec: example
      stillYaml: true
      usesSameSpecConvention: "not necessary"
spec:
  group: samples.crossplane.io
  names:
    kind: Mytype
    plural: mytypes
  scope: Namespaced
  version: v1alpha1
```

## Dependency Resolution

When a Stack requires types and functionality defined by another Stack, this dependency needs to be resolved and fulfilled.
All dependencies are expressed by the CRDs that are required, as opposed to the Stack or package that defines them.
Packages are units of publishing, pulling and versioning, they are not a unit of consumption.

Therefore, If the required CRDs don’t exist, the registry must be queried for what package defines them.
The registry maintains an index of package contents so that it can easily answer questions like this.
The full set of packages and Stacks that a Stack depend on will be downloaded, unpacked and installed before installation of the Stack itself can proceed.

## Package Processing

The process of installing a Stack involves downloading its package, extracting the package contents into its relevant CRDs and `Stack` record, and applying them to the Crossplane cluster.

See the [Installation Flow](#installation-flow) for a more complete view of the current package processing implementation.

The extension manager uses a "Job initContainer and shared volume" approach which copies the package contents from the package initContainer to a shared volume.  The SM, using command arguments to the Crossplane container, performs processing logic over the shared volume contents. The artifacts of this are sent to `stdout` where the main entry-point of the Crossplane container parses the unpacking container's `stdout`.  The parsed artifacts are then sent to the Kubernetes API for install.

The processing/unpacking logic can easily move to its own image that can be used as a base layer in the future (or a CLI tool).  This approach is not very divergent from the current implementation which divides these functions through the use of image entry-point command arguments.

A key aspect of the current implementation is that it takes advantage of the existing machinery in Kubernetes around container image download, verification, and extraction.
This is much more efficient and reliable than writing new code within Crossplane to do this work.

### Packaging Considerations

All packages should strive to minimize the amount of YAML they require.  Packages should avoid including any files that are not necessary for the Stack metadata or the controller, when including the controller in the same image.

Some packaging considerations do not need to be enforced by spec and are left to the Stack developer.

* Stack metadata may be bundled with the controller image, but this does not need to be the case nor should it be enforced one way or the other.
  * There may be benefits to metadata-only Stack images and their small byte sizes.
  * There is a benefit to requiring less image fetching by the container runtime
  * There may be benefits to maintaining fewer images (combined Stack metadata + controller image vs separate Stack metadata and controller images)

### Alternate Packaging Designs

Alternative designs for package processing and their related considerations (pros and cons) are listed below.  These ideas or parts of them may surface in future implementations.

* Stack package base image
  * A base image containing the unpacking logic could be defined and all other Stack packages are based on it, e.g., `FROM crossplane/stack`
  * The main entry point of the package would call this unpacking logic and send all artifacts to `stdout`
  * PRO: The knowledge to unpack an image is self-contained an external entity such as the SM does not need to know these details, package format is opaque to the SM.
  * CON: This likely significantly increases the size of the package if the logic is written in golang using Crossplane types for unmarshalling, increasing the KB size of the original package contents into an image that is many MB in size.
* Job initContainer and shared volume
  * Package images only contain the package contents, no processing logic is included in the package
  * The SM starts a job pod with an init container for the package image and copies all its contents to a shared volume in the job pod.  The Crossplane package processing logic runs in the main pod container and runs over the shared volume, sending all artifacts to `stdout`.
  * PRO: Package images are significantly smaller since they will only contain yaml files and icons, no binaries or libraries.
  * PRO: The processing/unpacking logic can easily move to its own image that can be used as a base layer in the future (or a CLI tool), so this approach is not very divergent.
  * CON: The SM needs to understand package content and layout.
* CLI tool
  * The unpacking logic could be built into a CLI tool that can be run over a package image.
  * PRO: This is the most versatile option as it can be used in contexts outside of a Crossplane cluster.
  * CON: Doesn't integrate very cleanly into the Crossplane mainline scenario with the SM.

Each of these designs offered a good place to start.  Through iteration over time we will learn more, hopefully without investing much effort that cannot be reused.

## Questions and Open Issues

* Stack Manager security model and isolation [#580](https://github.com/crossplaneio/crossplane/issues/580)
* Offloading redundant Stack Manager functionality to Stack building tools
* Dependency resolution design: [#434](https://github.com/crossplaneio/crossplane/issues/434)
* Updating/Upgrading Stack: [#435](https://github.com/crossplaneio/crossplane/issues/435)
* Support installation of stacks from private registries [#505](https://github.com/crossplaneio/crossplane/issues/505)
* Figure out model for crossplane core vs stacks [#531](https://github.com/crossplaneio/crossplane/issues/531)
* Single stack should be able to install multiple controllers [#532](https://github.com/crossplaneio/crossplane/issues/532)
* Prototype alternate stack implementations [#548](https://github.com/crossplaneio/crossplane/issues/548)
* Is there a benefit to `kind.version.` prefixed `crd.yaml` filenames
* What categories are valid? Is there a well-defined Category tree? Are arbitrary categories invalid or ignored?
* Should links be predefined (`website`, `source`) or freeform `links:[{description:"Website",url:"..."}, ...]`?
