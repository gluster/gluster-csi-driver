# CSI glusterfs driver


## Kubernetes
### Requirements

The folllowing feature gates and runtime config have to be enabled to deploy the driver

```
FEATURE_GATES=CSIPersistentVolume=true,MountPropagation=true
RUNTIME_CONFIG="storage.k8s.io/v1alpha1=true"
```

Mountprogpation requries support for privileged containers. So, make sure privileged containers are enabled in the cluster.

### Example local-up-cluster.sh

```ALLOW_PRIVILEGED=true FEATURE_GATES=CSIPersistentVolume=true,MountPropagation=true RUNTIME_CONFIG="storage.k8s.io/v1alpha1=true" LOG_LEVEL=5 hack/local-up-cluster.sh```

### Deploy

```kubectl -f deploy/kubernetes create```

### Example Nginx application
Please update the glusterfs Server & share information in nginx.yaml file.

```kubectl -f examples/kubernetes/nginx.yaml create```

## Using CSC tool

### Start glusterfs driver
```
$ sudo ./_output/glusterfsplugin --endpoint tcp://127.0.0.1:20000 --nodeid CSINode -v=5
```

## Test
Get ```csc``` tool from https://github.com/thecodeteam/gocsi/tree/master/csc

#### Get plugin info
```
$ csc identity plugin-info --endpoint tcp://127.0.0.1:20000
"GlusterFS"	"0.1.0"
```

### Get supported versions
```
$ csc identity supported-versions --endpoint tcp://127.0.0.1:20000
0.1.0
```

#### NodePublish a volume
```
$ export GLUSTERFS_SERVER="Your Server IP (Ex: 10.10.10.10)"
$ export GLUSTERFS_SHARE="Your GlusterFS share"
$ csc node publish --endpoint tcp://127.0.0.1:20000 --target-path /mnt/glusterfs --attrib server=$GLUSTERFS_SERVER --attrib share=$GLUSTERFS_SHARE demovol
demovol
```

#### NodeUnpublish a volume
```
$ csc node unpublish --endpoint tcp://127.0.0.1:20000 --target-path /mnt/glusterfs demovol
demovol
```

#### Get NodeID
```
$ csc node get-id --endpoint tcp://127.0.0.1:20000
CSINode
```

