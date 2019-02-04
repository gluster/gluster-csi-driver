# Automated testing

This document provides an overview of the automated testing for the Gluster-CSI
repository.

## End-to-end testing

(Full e2e testing is coming soon...)

## Testing of PRs

Pull requests are automatically tested by [Travis CI](https://travis-ci.com).
The tests that are run include:

- Running linters over the text-like files in the repo (bash, md, yaml, etc.)
- Building the code with recent versions of golang
- Running a number of code linters over the source (via [gometalinter](https://github.com/alecthomas/gometalinter))
- Building the container images via Docker

### Configuration

The configuration for Travis is controlled by the
[.travis.yml](https://github.com/gluster/gluster-csi-driver/blob/master/.travis.yml)
file in the main directory of the repo. Briefly, there is one job for each type
of CSI driver. Within each job, the "install" steps are executed in order,
followed by the "script" steps. If the CI job is running against `master` or a
tag of the form 'v[0-9]+', the deploy step will execute, pushing the built
containers to both Quay and Docker hub. If any command fails, the job will
register a failure. See the main Travis documentation for details on the
configuration file.

### Troubleshooting

If your PR fails to build, the best place to start is the logs from Travis.
Head over to the [list of PR
jobs](https://travis-ci.com/gluster/gluster-csi-driver/pull_requests) and find
yours. Once you locate your job, you can examine the individual test jobs and
see which one(s) failed. By clicking on it, you can view the job output and
diagnose the failure.

When looking for failures, remember that the job will continue to run even
after the first failure is detected. A failed command may actually be the
result of an earlier command failing, so check through the entire output, not
just the last couple lines.
