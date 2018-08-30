# Gluster Volume as persistent store for running Redis server

## Create PersistentVolumeClaim for redis server

Create the yaml file(`volume-redis-app.yaml`) as below by specifying
the claim size as required

```
---
kind: PersistentVolumeClaim
apiVersion: v1
metadata:
  name: redis-server-vol
  annotations:
    volume.beta.kubernetes.io/storage-class: glusterfs-csi
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
```

Then run `kubectl` command to create the Volume as below

```
[root@localhost cluster]# kubectl create -f volume-redis-app.yaml
```

Once the Volume is created, create the redis pod yaml
file(`redis-app.yml`) with the Volume claim

```
---
apiVersion: v1
kind: Pod
metadata:
  name: redis-app
  labels:
    name: redis-app
spec:
  containers:
  - name: redis-app
    image: redis
    imagePullPolicy: IfNotPresent
    volumeMounts:
    - mountPath: "/data"
      name: glustercsivol
  volumes:
  - name: glustercsivol
    persistentVolumeClaim:
      claimName: redis-server-vol
```

Use `kubectl` command to create redis pod

```
[root@localhost cluster]# kubectl create -f redis-app.yaml
```

Login to the `redis-app` pod using the following command,

```
[root@localhost cluster]# kubectl exec -it redis-app /bin/bash
```

and set a value using `redis-cli` as below

```
root@redis-app:/data# redis-cli
127.0.0.1:6379> set welcome-msg "Hello World"
OK
127.0.0.1:6379> get welcome-msg
"Hello World"
127.0.0.1:6379> save
OK
127.0.0.1:6379> exit
root@redis-app:/data#
```

Now restart the redis pod and see the values set in redis persists.

```
[root@localhost cluster]# kubectl delete pods redis-app
[root@localhost cluster]# kubectl create -f redis-app.yaml
```

```
[root@master ~]# kubectl exec -it redis-app /bin/bash
root@redis-app:/data# redis-cli
127.0.0.1:6379> get welcome-msg
"Hello World"
```
