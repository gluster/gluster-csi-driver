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
  volumeType:
    type: replicate
    replicate:
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
  volumeType:
    type: replicate
    replicate:
      replicas: 3
      arbiterType: normal
  volumeOptions:
    "performance.cache-refresh-timeout": "60"
    "performance.cache-size": "134217728"
    "performance.nl-cache": "on"
    "performance.md-cache-timeout": "300"
```

## Erasure coded volume

This defines a template for 4+2 erasure coded volumes. Additionally, the bricks
will be spread across three specific availability zones. (Gluster nodes are
tagged with the AZ in which they reside).

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: simpleR3
provisioner: org.gluster.glusterfs
parameters:
  volumeType:
    type: disperse
    disperse:
      data: 4
      redundancy: 2
  volumeLayout:
    dataZones:
      - rack1
      - rack2
      - rack7
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
  volumeType:
    # Type of volume (required): replicate | disperse
    type: replicate
    replicate: # if type == replicate
      replicas: 3
      # Type of arbiter (default=none): none | normal | thin
      arbiterType: thin
      halo: false
      haloOptions:
        latency: 2
        minReplicas: 2
    disperse: # if type == disperse
      data: 4
      redundancy: 2
  volumeOptions:
    # Arbitrary gluster options to set on the volume. If you would do
    # "gluster vol set myvol foo bar", you would put foo:bar here.
    # this is just an example...
    "performance.md-cache-timeout": "300"
  # PV capacity * capacityReservation will be reserved for use by this
  # volume. The reservation should account for snaps, clones, compression,
  # and deduplication. Float >= 0
  capacityReservation: "1.0"
  # Either a list of failure domains to use or '*' for any. Bricks will be
  # spread across the listed domains.
  volumeLayout:
    # A list of zone(s) to be used for this volume type. Omitting the list
    # allows all zones to be used (i.e., default=*)
    dataZones:
      - az-1
    # By default, dataZones will be used for all bricks. Supplying
    # arbiterZones restricts the placement of the arbiter to the provided
    # zone(s). Only applicable when volumeType.type==replicate &&
    # volumeType.replicate.arbiterType!=none
    arbiterZones:
      - az-2
```
