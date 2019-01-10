# Running the Tests
1. Setup a Kubernetes Cluster and setup GCS inside it as the [GCS deployment
   scripts recommend][1].
2. Make sure `kubectl` points to that cluster by doing the following:

```
    KUBECONFIG=/path/to/kubeconfig
```
3. Install dependencies with `dep ensure`.
4. Change into this directory `cd test/e2e`.
4. Run the tests with `go test`.


[1]: https://github.com/gluster/gcs/tree/master/deploy
