# gluster-csi-driver

[![Go Report Card](https://goreportcard.com/badge/github.com/gluster/gluster-csi-driver)](https://goreportcard.com/report/github.com/gluster/gluster-csi-driver)
[![Build Status](https://ci.centos.org/view/Gluster/job/gluster_csi-driver-master/badge/icon)](https://ci.centos.org/view/Gluster/job/gluster_csi-driver-master/)

This repo contains CSI driver for Gluster. The Container Storage Interface
(CSI) is a proposed new industry standard for cluster-wide volume plugins.
“Container Storage Interface” (CSI)  enables storage vendors (SP) to develop a
plugin once and have it work across a number of container orchestration (CO)
systems.

## Demo of GlusterFS CSI driver to create and delete volumes on GD2 Cluster

[![GlusterFS CSI driver Demo](https://asciinema.org/a/195024.png)](https://asciinema.org/a/195024)

## Building GlusterFS CSI driver

This repository contains the source and a Dockerfile to build the GlusterFS CSI
driver. The driver is built as a multi-stage container build. This requires a
relatively recent version of Docker or Buildah.

Docker packages can be obtained for
[CentOS](https://docs.docker.com/install/linux/docker-ce/centos/),
[Fedora](https://docs.docker.com/install/linux/docker-ce/fedora/) or other
distributions.

To build, ensure docker is installed, and run:

1. Get inside the repository directory

```
[root@localhost]# cd gluster-csi-driver
```

1. Build the glusterfs-csi-driver container

```
[root@localhost]# ./build.sh
```

## Testing GlusterFS CSI driver

### Deploy kubernetes Cluster

### Deploy a GD2 gluster cluster

### Deploy CSI driver

```
[root@localhost]#kubectl create -f csi-deployment.yaml
service/csi-attacher-glusterfsplugin created
statefulset.apps/csi-attacher-glusterfsplugin created
daemonset.apps/csi-nodeplugin-glusterfsplugin created
service/csi-provisioner-glusterfsplugin created
statefulset.apps/csi-provisioner-glusterfsplugin created
serviceaccount/glusterfs-csi created
clusterrole.rbac.authorization.k8s.io/glusterfs-csi created
clusterrolebinding.rbac.authorization.k8s.io/glusterfs-csi-role created
```

Below listed feature gates need to be enabled in kubernetes v1.13.1

```
--feature-gates=VolumeSnapshotDataSource=true
```

>NOTE: You can skip seperate installation of kubernetes cluster, GD2 Cluster
and CSI deployment if you directly use [GCS](https://github.com/gluster/gcs)
installation method, it should bring your deployment in one shot. Refer
[GCS deployment guide](https://github.com/gluster/gcs/blob/master/deploy/README.md)
for more details.

### Create a storage class

```
[root@localhost]# cat storage-class.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: glusterfs-csi
  annotations:
    storageclass.kubernetes.io/is-default-class: "true"
provisioner: org.gluster.glusterfs
```

```
[root@localhost]# kubectl create -f storage-class.yaml
storageclass.storage.k8s.io/glusterfs-csi created
```

Verify storage class

```
[root@localhost]# kubectl get storageclass
NAME                      PROVISIONER             AGE
glusterfs-csi (default)   org.gluster.glusterfs   105s
[root@localhost]# kubectl describe storageclass/glusterfs-csi
Name:                  glusterfs-csi
IsDefaultClass:        Yes
Annotations:           storageclass.kubernetes.io/is-default-class=true
Provisioner:           org.gluster.glusterfs
Parameters:            <none>
AllowVolumeExpansion:  <unset>
MountOptions:          <none>
ReclaimPolicy:         Delete
VolumeBindingMode:     Immediate
Events:                <none>
```

### Create PersistentVolumeClaim

```
[root@localhost]# cat pvc.yaml
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: glusterfs-csi-pv
spec:
  storageClassName: glusterfs-csi
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 5Gi

[root@localhost cluster]# kubectl create -f pvc.yaml
persistentvolumeclaim/glusterfs-csi-pv created
```

Validate the claim creation

```
[root@localhost]# kubectl get pvc
NAME      STATUS    VOLUME                                                        CAPACITY   ACCESS MODES   STORAGECLASS   AGE
glusterfs-csi-pv   Bound     pvc-953d21f5a51311e8   5Gi        RWX            glusterfs-csi   3s
```

```
[root@localhost]# kubectl describe pvc
Name:          glusterfs-csi-pv
Namespace:     default
StorageClass:  glusterfs-csi
Status:        Bound
Volume:        pvc-953d21f5a51311e8
Labels:        <none>
Annotations:   control-plane.alpha.kubernetes.io/leader={"holderIdentity":"874a6cc9-a511-11e8-bae2-0a580af40202","leaseDurationSeconds":15,"acquireTime":"2018-08-21T07:26:58Z","renewTime":"2018-08-21T07:27:00Z","lea...
               pv.kubernetes.io/bind-completed=yes
               pv.kubernetes.io/bound-by-controller=yes
               storageClassName=glusterfs-csi
               volume.kubernetes.io/storage-provisioner=org.gluster.glusterfs
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      5Gi
Access Modes:  RWX
Events:
  Type    Reason                 Age                From                                                                                          Message
  ----    ------                 ----               ----                                                                                          -------
  Normal  ExternalProvisioning   30s (x2 over 30s)  persistentvolume-controller                                                                   waiting for a volume to be created, either by external provisioner "org.gluster.glusterfs" or manually created by system administrator
  Normal  Provisioning           30s                org.gluster.glusterfs csi-provisioner-glusterfsplugin-0 874a6cc9-a511-11e8-bae2-0a580af40202  External provisioner is provisioning volume for claim "default/glusterfs-csi-pv"
  Normal  ProvisioningSucceeded  29s                org.gluster.glusterfs csi-provisioner-glusterfsplugin-0 874a6cc9-a511-11e8-bae2-0a580af40202  Successfully provisioned volume pvc-953d21f5a51311e8
```

Verify PV details:

```
[root@localhost]# kubectl describe pv
Name:            pvc-953d21f5a51311e8
Labels:          <none>
Annotations:     pv.kubernetes.io/provisioned-by=org.gluster.glusterfs
Finalizers:      [kubernetes.io/pv-protection]
StorageClass:    glusterfs-csi
Status:          Bound
Claim:           default/glusterfs-csi-pv
Reclaim Policy:  Delete
Access Modes:    RWX
Capacity:        5Gi
Node Affinity:   <none>
Message:
Source:
    Type:          CSI (a Container Storage Interface (CSI) volume source)
    Driver:        org.gluster.glusterfs
    VolumeHandle:  pvc-953d21f5a51311e8
    ReadOnly:      false
Events:            <none>
```

### Create a pod with this claim

```
[root@localhost]# cat app.yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: gluster
  labels:
    name: gluster
spec:
  containers:
  - name: gluster
    image: redis
    imagePullPolicy: IfNotPresent
    volumeMounts:
    - mountPath: "/mnt/gluster"
      name: glustercsivol
  volumes:
  - name: glustercsivol
    persistentVolumeClaim:
      claimName: glusterfs-csi-pv

[root@localhost cluster]#kubectl create -f app.yaml
```

Check mount output and validate.

```
[root@localhost]# mount |grep glusterfs
192.168.121.158:pvc-953d21f5a51311e8 on /var/lib/kubelet/pods/2a563343-a514-11e8-a324-525400a04cb4/volumes/kubernetes.io~csi/pvc-953d21f5a51311e8/mount type fuse.glusterfs (rw,relatime,user_id=0,group_id=0,default_permissions,allow_other,max_read=131072)

[root@localhost]# kubectl delete pod gluster
pod "gluster" deleted
[root@localhost]# mount |grep glusterfs
[root@localhost]#
```

### Support for Snapshot

Kubernetes v1.12 introduces alpha support for volume snapshotting.
This feature allows creating/deleting volume snapshots, and the ability
to create new volumes from a snapshot natively using the Kubernetes API.

To verify clone functionality work as intended,
lets start with writing some data into already created application with PVC.

```
[root@localhost]# kubectl exec -it redis /bin/bash
root@redis:/data# cd /mnt/gluster/
root@redis:/mnt/gluster# echo "glusterfs csi clone test" > clone_data
```

### Create a snapshot class

```
[root@localhost]# cat snapshot-class.yaml
---
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshotClass
metadata:
  name: glusterfs-csi-snap
snapshotter: org.gluster.glusterfs
```

```
[root@localhost]#kubectl create -f snapshot-class.yaml
volumesnapshotclass.snapshot.storage.k8s.io/glusterfs-csi-snap created
```

Verify snapshot class

```
[root@localhost]# kubectl get volumesnapshotclass
NAME               AGE
glusterfs-csi-snap   1h
[root@localhost]# kubectl describe volumesnapshotclass/glusterfs-csi-snap
Name:         glusterfs-csi-snap
Namespace:
Labels:       <none>
Annotations:  <none>
API Version:  snapshot.storage.k8s.io/v1alpha1
Kind:         VolumeSnapshotClass
Metadata:
  Creation Timestamp:  2018-10-24T04:57:34Z
  Generation:          1
  Resource Version:    3215
  Self Link:           /apis/snapshot.storage.k8s.io/v1alpha1/volumesnapshotclasses/glusterfs-csi-snap
  UID:                 51de83df-d749-11e8-892a-525400d84c47
Snapshotter:           org.gluster.glusterfs
Events:                <none>
```

### Create a snapshot from pvc

```
[root@localhost]# cat volume-snapshot.yaml
---
apiVersion: snapshot.storage.k8s.io/v1alpha1
kind: VolumeSnapshot
metadata:
  name: glusterfs-csi-ss
spec:
  snapshotClassName: glusterfs-csi-ss
  source:
    name: glusterfs-csi-pv
    kind: PersistentVolumeClaim

```

```
[root@localhost]# kubectl create -f volume-snapshot.yaml
volumesnapshot.snapshot.storage.k8s.io/glusterfs-csi-ss created
```

Verify volume snapshot

```
[root@localhost]# kubectl get volumesnapshot
NAME               AGE
glusterfs-csi-ss   13s
[root@localhost]# kubectl describe volumesnapshot/glusterfs-csi-ss
Name:         glusterfs-csi-ss
Namespace:    default
Labels:       <none>
Annotations:  <none>
API Version:  snapshot.storage.k8s.io/v1alpha1
Kind:         VolumeSnapshot
Metadata:
  Creation Timestamp:  2018-10-24T06:39:35Z
  Generation:          1
  Resource Version:    12567
  Self Link:           /apis/snapshot.storage.k8s.io/v1alpha1/namespaces/default/volumesnapshots/glusterfs-csi-ss
  UID:                 929722b7-d757-11e8-892a-525400d84c47
Spec:
  Snapshot Class Name:    glusterfs-csi-snap
  Snapshot Content Name:  snapcontent-929722b7-d757-11e8-892a-525400d84c47
  Source:
    Kind:  PersistentVolumeClaim
    Name:  glusterfs-csi-pv
Status:
  Creation Time:  1970-01-01T00:00:01Z
  Ready:          true
  Restore Size:   <nil>
Events:           <none>
```

### Provision new volume from snapshot

```
[root@localhost]# cat pvc-restore.yaml
---
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: glusterfs-pv-restore
spec:
  storageClassName: glusterfs-csi
  dataSource:
    name: glusterfs-csi-ss
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
```

```
[root@localhost]# kubectl create -f pvc-restore.yaml
persistentvolumeclaim/glusterfs-pv-restore created
```

Verify newly created claim

```
[root@localhost]# kubectl get pvc
NAME                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS    AGE
glusterfs-csi-pv       Bound    pvc-712278b0-d749-11e8-892a-525400d84c47   5Gi        RWX            glusterfs-csi   103m
glusterfs-pv-restore   Bound    pvc-dfcc36f0-d757-11e8-892a-525400d84c47   5Gi        RWO            glusterfs-csi   14s
```

```
[root@localhost]# kubectl describe pvc/glusterfs-pv-restore
Name:          glusterfs-pv-restore
Namespace:     default
StorageClass:  glusterfs-csi
Status:        Bound
Volume:        pvc-dfcc36f0-d757-11e8-892a-525400d84c47
Labels:        <none>
Annotations:   pv.kubernetes.io/bind-completed: yes
               pv.kubernetes.io/bound-by-controller: yes
               volume.kubernetes.io/storage-provisioner: org.gluster.glusterfs
Finalizers:    [kubernetes.io/pvc-protection]
Capacity:      5Gi
Access Modes:  RWO
Events:
  Type       Reason                 Age   From                                                                                          Message
  ----       ------                 ----  ----                                                                                          -------
  Normal     ExternalProvisioning   41s   persistentvolume-controller                                                                   waiting for a volume to be created, either by external provisioner "org.gluster.glusterfs" or manually created by system administrator
  Normal     Provisioning           41s   org.gluster.glusterfs_csi-provisioner-glusterfsplugin-0_1e7821cb-d749-11e8-9935-0a580af40303  External provisioner is provisioning volume for claim "default/glusterfs-pv-restore"
  Normal     ProvisioningSucceeded  41s   org.gluster.glusterfs_csi-provisioner-glusterfsplugin-0_1e7821cb-d749-11e8-9935-0a580af40303  Successfully provisioned volume pvc-dfcc36f0-d757-11e8-892a-525400d84c47
Mounted By:  <none>
```

### Create an app with New claim

```
[root@localhost]# cat app-with-clone.yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: redis-pvc-restore
  labels:
    name: redis-pvc-restore
spec:
  containers:
    - name: redis-pvc-restore
      image: redis:latest
      imagePullPolicy: IfNotPresent
      volumeMounts:
        - mountPath: "/mnt/gluster"
          name: glusterfscsivol
  volumes:
    - name: glusterfscsivol
      persistentVolumeClaim:
        claimName: glusterfs-pv-restore

```

```

[root@localhost]# kubectl create -f app-with-clone.yaml
pod/redis-pvc-restore created
```

Verify cloned data is present in newly created application

```
[root@localhost]# kubectl get po
NAME                                   READY   STATUS    RESTARTS   AGE
csi-attacher-glusterfsplugin-0         2/2     Running   0          112m
csi-nodeplugin-glusterfsplugin-dl7pp   2/2     Running   0          112m
csi-nodeplugin-glusterfsplugin-khrtd   2/2     Running   0          112m
csi-nodeplugin-glusterfsplugin-kqcsw   2/2     Running   0          112m
csi-provisioner-glusterfsplugin-0      3/3     Running   0          112m
glusterfs-55v7v                        1/1     Running   0          128m
glusterfs-qbvgv                        1/1     Running   0          128m
glusterfs-vclr4                        1/1     Running   0          128m
redis                                  1/1     Running   0          109m
redis-pvc-restore                      1/1     Running   0          26s
[root@master vagrant]# kubectl exec -it redis-pvc-restore /bin/bash
root@redis-pvc-restore:/data# cd /mnt/gluster/
root@redis-pvc-restore:/mnt/gluster# ls
clone_data
root@redis-pvc-restore:/mnt/gluster# cat clone_data
glusterfs csi clone test
```

#### Create PVC with thin arbiter support

follow [guide](
  https://docs.gluster.org/en/latest/Administrator%20Guide/Thin-Arbiter-Volumes/)
to setup thin arbiter

### Create Thin-Arbiter storage class

```
$ cat thin-arbiter-storageclass.yaml
---
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: glusterfs-csi-thin-arbiter
provisioner: org.gluster.glusterfs
parameters:
  arbiterType: "thin"
  arbiterPath: "192.168.10.90:24007/mnt/arbiter-path"
```

```
$ kubectl create -f thin-arbiter-storageclass.yaml
storageclass.storage.k8s.io/glusterfs-csi-thin-arbiter created
```

### Create Thin-Arbiter PersistentVolumeClaim

```
$ cat thin-arbiter-pvc.yaml
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: glusterfs-csi-thin-pv
spec:
  storageClassName: glusterfs-csi-thin-arbiter
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 5Gi
```

```
$ kube create -f thin-arbiter-pvc.yaml
persistentvolumeclaim/glusterfs-csi-thin-pv created
```

Verify PVC is in Bound state

```
$ kube get pvc
NAME                     STATUS        VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS            AGE
glusterfs-csi-thin-pv    Bound         pvc-86b3b70b-1fa0-11e9-9232-525400ea010d   5Gi        RWX            glusterfs-csi-arbiter   13m

```

### Create an app with claim

```
$ cat thin-arbiter-pod.yaml
---
apiVersion: v1
kind: Pod
metadata:
  name: ta-redis
  labels:
    name: redis
spec:
  containers:
    - name: redis
      image: redis
      imagePullPolicy: IfNotPresent
      volumeMounts:
        - mountPath: "/mnt/gluster"
          name: glusterfscsivol
  volumes:
    - name: glusterfscsivol
      persistentVolumeClaim:
        claimName: glusterfs-csi-thin-pv
```

```
$ kube create -f thin-arbiter-pod.yaml
pod/ta-redis created
```

Verify app is in running state

```
$ kube get po
NAME        READY   STATUS        RESTARTS   AGE
ta-redis    1/1     Running       0          6m54s
```
