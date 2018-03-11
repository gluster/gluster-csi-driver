# gluster-csi-driver

This repo contains CSI driver for Gluster. The Container Storage Interface (CSI) is a proposed new industry standard for cluster-wide volume plugins.  “Container Storage Interface” (CSI)  enables storage vendors (SP) to develop a plugin once and have it work across a number of container orchestration (CO) systems. 

## Testing GlusterFS CSI driver

### Create a storage class
~~~
[root@localhost cluster]# cat csi-sc.yaml 

apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: glusterfscsi
  annotations:
    storageclass.beta.kubernetes.io/is-default-class: "true"
provisioner: org.gluster.glusterfs
[root@localhost cluster]# 

[root@localhost cluster]# kubectl create -f csi-sc.yaml
~~~


### Create PersistentVolumeClaim
~~~
[root@localhost cluster]# cat glusterfs-pvc-claim1_fast.yaml 
{
   "kind": "PersistentVolumeClaim",
   "apiVersion": "v1",
   "metadata": {
     "name": "claim1",
     "annotations": {
     "volume.beta.kubernetes.io/storage-class": "glusterfscsi"
     }
   },
   "spec": {
     "accessModes": [
       "ReadWriteMany"
     ],
    "resources": {
       "requests": {
         "storage": "4Gi"
       }
     }
   }
}

[root@localhost cluster]# kubectl create -f glusterfs-pvc-claim1_fast.yaml
~~~
Validate the claim creation

~~~
[root@localhost cluster]# kubectl get pvc
NAME      STATUS    VOLUME                                                        CAPACITY   ACCESS MODES   STORAGECLASS   AGE
claim1   Bound     kubernetes-dynamic-pvc-ad8014ec-febd-11e7-bf55-c85b7636c232   4Gi        RWX            glusterfscsi   35m
[root@localhost cluster]# 
~~~

~~~
[root@localhost kubernetes]# kubectl describe pvc
Name:          claim1
Namespace:     default
StorageClass:  glusterfscsi
Status:        Bound
Volume:        kubernetes-dynamic-pvc-79eb02cd-fd17-11e7-ac3c-c85b7636c232
Labels:        <none>
Annotations:   control-plane.alpha.kubernetes.io/leader={"holderIdentity":"79e518d1-fd17-11e7-ac3c-c85b7636c232","leaseDurationSeconds":15,"acquireTime":"2018-01-19T12:51:32Z","renewTime":"2018-01-19T12:51:34Z","lea...
               pv.kubernetes.io/bind-completed=yes
               pv.kubernetes.io/bound-by-controller=yes
               volume.beta.kubernetes.io/storage-class=glusterfscsi
               volume.beta.kubernetes.io/storage-provisioner=org.gluster.glusterfs
Finalizers:    []
Capacity:      4Gi
Access Modes:  RWX
Events:
  Type    Reason                 Age              From                                                                            Message
  ----    ------                 ----             ----                                                                            -------
  Normal  ExternalProvisioning   5m (x7 over 6m)  persistentvolume-controller                                                     waiting for a volume to be created, either by external provisioner "org.gluster.glusterfs" or manually created by system administrator
  Normal  Provisioning           5m               org.gluster.glusterfs localhost.localdomain 79e518d1-fd17-11e7-ac3c-c85b7636c232  External provisioner is provisioning volume for claim "default/claim1"
  Normal  ProvisioningSucceeded  5m               org.gluster.glusterfs localhost.localdomain 79e518d1-fd17-11e7-ac3c-c85b7636c232  Successfully provisioned volume kubernetes-dynamic-pvc-79eb02cd-fd17-11e7-ac3c-c85b7636c232
~~~


Verify PV details:

~~~
[root@localhost cluster]# kubectl describe pv
Name:            kubernetes-dynamic-pvc-ad8014ec-febd-11e7-bf55-c85b7636c232
Labels:          <none>
Annotations:     csi.volume.kubernetes.io/volume-attributes={"glusterserver":"172.18.0.3","glustervol":"vol_64d3ac458bc17bec44a919336656fbfb"}
                 csiProvisionerIdentity=1516547610828-8081-org.gluster.glusterfs
                 pv.kubernetes.io/provisioned-by=org.gluster.glusterfs
StorageClass:    glusterfscsi
Status:          Bound
Claim:           default/claim1
Reclaim Policy:  Delete
Access Modes:    RWX
Capacity:        4Gi
Message:
Source:
    Type:          CSI (a Container Storage Interface (CSI) volume source)
    Driver:        org.gluster.glusterfs
    VolumeHandle:  ad817b9d-febd-11e7-96d9-c85b7636c232
    ReadOnly:      false
Events:            <none>
[root@localhost cluster]# 
~~~


### Create a pod with this claim
~~~
[root@localhost cluster]# cat ../demo/fedora-pod.json
{
    "apiVersion": "v1",
    "kind": "Pod",
    "metadata": {
        "name": "fedora",
        "labels": {
            "name": "fedora"
        }
    },
    "spec": {
        "containers": [{
            "name": "fedora",
            "image": "fedora",
            "imagePullPolicy": "IfNotPresent",
            "volumeMounts": [{
                "mountPath": "/mnt/gluster",
                "name": "glustercsivol"
            }]
        }],
       "volumes": [{
            "name": "glustercsivol",
            "persistentVolumeClaim": {
                "claimName": "claim1"
            }
        }]
    }
}


[root@localhost cluster]#kubectl create -f demo/fedora-pod.json
~~~

Check mount output and validate.
~~~

[root@localhost cluster]# mount |grep gluster
172.18.0.3:vol_64d3ac458bc17bec44a919336656fbfb on /var/lib/kubelet/pods/e6476013-febd-11e7-bde6-c85b7636c232/volumes/kubernetes.io~csi/kubernetes-dynamic-pvc-ad8014ec-febd-11e7-bf55-c85b7636c232/mount type fuse.glusterfs (rw,relatime,user_id=0,group_id=0,default_permissions,allow_other,max_read=131072)

[root@localhost cluster]# kubectl delete pod gluster
pod "gluster" deleted
[root@localhost cluster]# mount |grep glusterfs
[root@localhost cluster]#
