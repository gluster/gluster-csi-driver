# Running the Tests

* Setup a Kubernetes Cluster and setup GCS inside it as the [GCS deployment
   scripts recommend][1].
* Make sure `kubectl` points to that cluster by doing the following:

```
    KUBECONFIG=/path/to/kubeconfig
```

* Install dependencies with `dep ensure -vendor-only`.
* Change into this directory `cd test/e2e`.
* Run the tests with `go test`.

[1]: https://github.com/gluster/gcs/tree/master/deploy
