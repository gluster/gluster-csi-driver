This document describes the design of the StorageClass parameters for the
file-based Gluster CSI driver, `org.gluster.glusterfs`. It is intended to
describe the desired interface for configuring Gluster-file based
persistent volume types.

This document presents a number of example StorageClass definitions, followed
by a full yaml description at the end of the document.

# Examples

## Simple replica 3

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: simpleR3
provisioner: org.gluster.glusterfs
parameters:
  replicas: 3
```

## Replica 3 with arbiter

This creates a volume with 2 data nodes and a 3rd brick that contains just file
metadata (the arbiter brick). This example also shows setting volume options.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: r3Arbiter
provisioner: org.gluster.glusterfs
parameters:
  replicas: 2
  arbiterType: normal
  volumeOptions: "performance.cache-refresh-timeout=60,performance.cache-size=134217728,performance.nl-cache=on,performance.md-cache-timeout=300"
```

## Erasure coded volume

This defines a template for 4+2 erasure coded volumes. Additionally, the bricks
will be spread across three specific availability zones. (Gluster nodes are
tagged with the AZ in which they reside).

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: ec
provisioner: org.gluster.glusterfs
parameters:
  disperseData: 4
  disperseRedundancy: 2
  dataZones: "rack1,rack2,rack7"
```

# Full yaml description

Below is the full description of the StorageClass, complete with all optional
fields.

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: reliable
provisioner: org.gluster.glusterfs
parameters:
  # Maximum size of an individual brick. Volumes that exceed this will be
  # created as distributed-* volumes.
  maxBrickSize: 100Gi
  # Volumes should be of type replicate w/ this many data replicas
  replicas: 3
  # Type of arbiter for replicate volumes. default=none
  arbiterType: none  # (none | normal | thin)
  # Whether halo replication whould be enabled. default=false
  halo: false  # (false | true)
  # The latency "halo" when using halo-based replication
  haloLatencyMs: 2
  # Minimum number of replicas updated synchronously when halo is enabled
  haloMinReplicas: 2
  # Volumes should be of type "disperse" w/ this many data fragments
  disperseData: 4
  # Number of parity fragments for disperse volumes
  disperseRedundancy: 2
  # Arbitrary gluster options to set on the volume. If you would do
  # "gluster vol set myvol foo bar", you would put foo=bar here. Multiple
  # options are comma separated.
  volumeOptions: "performance.md-cache-timeout=300,other.option=7"
  # PV capacity * capacityReservation will be reserved for use by this
  # volume. The reservation should account for snaps, clones, compression,
  # and deduplication. Float >= 0
  capacityReservation: 1.0
  # A list of zone(s) to be used for this volume type. Replica data, disperse
  # data, and disperse redundancy bricks will be placed in the listed zones.
  # Omitting the list allows all zones to be used (i.e., default=*)
  dataZones: "az1,az2,az3"
  # Supplying arbiterZones restricts the placement of arbiter bricks to the
  # provided zone(s). Only applicable for replicate volumes with arbiter. If
  # not specified, dataZones will be used.
  arbiterZones: "az4,az5"
```

The type of volume (replicate or disperse) is chosen based on whether
`replicas` or `disperseData` + `disperseRedundancy` are specified. It is an
error to specify both.

Arbiter-related options (`arbiterType` and `arbiterZones`) may only be
specified for replicate volumes, and `arbiterZones` may only be specified if
the `arbiterType` is not `none`.

Halo options are only valid for replicate volumes. `haloLatencyMs` and
`haloMinReplicas` may only be specified if `halo` is `true`.
