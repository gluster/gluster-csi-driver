# gluster-csi-driver

[![Build Status](https://ci.centos.org/view/Gluster/job/gluster_csi-driver-smoke/badge/icon)](https://ci.centos.org/view/Gluster/job/gluster_csi-driver-smoke/)

This repo contains CSI driver for Gluster. The Container Storage Interface
(CSI) is a proposed new industry standard for cluster-wide volume plugins.
“Container Storage Interface” (CSI)  enables storage vendors (SP) to develop a
plugin once and have it work across a number of container orchestration (CO)
systems.

## Demo of GlusterFS CSI driver to create and delete volumes on GD2 Cluster

[![GlusterFS CSI driver Demo](https://asciinema.org/a/195024.png)](https://asciinema.org/a/195024)

## Building GlusterFS CSI driver

This repository consists of Dockerfile for GlusterFS CSI dirver to build on
CentOS distribution. Once you clone the repository, to build the image, run the
following command:

1. Get inside the repository directory

```
[root@localhost]#cd gluster-csi-driver
```

1. Compile and create a binary

```
[root@localhost]#make
```

3) Build a docker image based on the binary compiled above

```
[root@localhost]#docker build -t glusterfs-csi-driver -f pkg/glusterfs/Dockerfile .
```

## Testing GlusterFS CSI driver

### Deploy kubernetes Cluster

### Deploy a GD2 gluster cluster

### Deploy CSI driver

```
[root@localhost cluster]#kubectl create -f csi-deployment.yaml
service/csi-attacher-glusterfsplugin created
statefulset.apps/csi-attacher-glusterfsplugin created
daemonset.apps/csi-nodeplugin-glusterfsplugin created
service/csi-provisioner-glusterfsplugin created
statefulset.apps/csi-provisioner-glusterfsplugin created
serviceaccount/glusterfs-csi created
clusterrole.rbac.authorization.k8s.io/glusterfs-csi created
clusterrolebinding.rbac.authorization.k8s.io/glusterfs-csi-role created
```

### Create a storage class

```
[root@localhost cluster]# cat sc.yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: glusterfs-csi
  annotations:
    storageclass.beta.kubernetes.io/is-default-class: "true"
provisioner: org.gluster.glusterfs
```

### Create PersistentVolumeClaim

```
[root@localhost cluster]# cat pvc.yaml
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: glusterfs-csi-pv
  annotations:
    volume.beta.kubernetes.io/storage-class: glusterfs-csi
spec:
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
[root@localhost cluster]# kubectl get pvc
NAME      STATUS    VOLUME                                                        CAPACITY   ACCESS MODES   STORAGECLASS   AGE
glusterfs-csi-pv   Bound     pvc-953d21f5a51311e8   5Gi        RWX            glusterfs-csi   3s
```

```
[root@localhost cluster]# kubectl describe pvc
Name:          glusterfs-csi-pv
Namespace:     default
StorageClass:  glusterfs-csi
Status:        Bound
Volume:        pvc-953d21f5a51311e8
Labels:        <none>
Annotations:   control-plane.alpha.kubernetes.io/leader={"holderIdentity":"874a6cc9-a511-11e8-bae2-0a580af40202","leaseDurationSeconds":15,"acquireTime":"2018-08-21T07:26:58Z","renewTime":"2018-08-21T07:27:00Z","lea...
               pv.kubernetes.io/bind-completed=yes
               pv.kubernetes.io/bound-by-controller=yes
               volume.beta.kubernetes.io/storage-class=glusterfs-csi
               volume.beta.kubernetes.io/storage-provisioner=org.gluster.glusterfs
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
[root@localhost cluster]# kubectl describe pv
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
[root@master vagrant]# cat app.yaml
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
[root@localhost cluster]# mount |grep glusterfs
192.168.121.158:pvc-953d21f5a51311e8 on /var/lib/kubelet/pods/2a563343-a514-11e8-a324-525400a04cb4/volumes/kubernetes.io~csi/pvc-953d21f5a51311e8/mount type fuse.glusterfs (rw,relatime,user_id=0,group_id=0,default_permissions,allow_other,max_read=131072)

[root@localhost cluster]# kubectl delete pod gluster
pod "gluster" deleted
[root@localhost cluster]# mount |grep glusterfs
[root@localhost cluster]#
```
