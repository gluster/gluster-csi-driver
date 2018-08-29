# Contributing Guide

Want to help the gluster-csi-driver project? This document covers some of the
policies and preferences contributors to the project need to
know about.

* [The Basics](#the-basics)
* [Contributor's Workflow](#contributors-workflow)

## The Basics

### New to Go?

gluster-csi-driver is primarily written in Go and if you are new to the language, it is *highly* encouraged you take [A Tour of Go](http://tour.golang.org/welcome/1).

### New to GitHub?

If you are new to the GitHub process, please see https://guides.github.com/introduction/flow/index.html.

### Getting Started

1. Fork the gluster-csi-project GitHub project
2. Download latest Go to your system
3. Setup your [GOPATH](http://www.g33knotes.org/2014/07/60-second-count-down-to-go.html) environment
4. Type: `mkdir -p $GOPATH/src/github.com/gluster`
5. Type: `cd $GOPATH/src/github.com/gluster`
6. Type: `git clone https://github.com/gluster/gluster-csi-driver.git`
7. Type: `cd gluster-csi-driver`

Now you need to setup your repo where you will be pushing your changes into:

1. `git remote add <rname> <your-github-fork>`
2. `git fetch <rname>`

Where `<rname>` is a remote name of your choosing and `<your-github-fork>`
is a git URL that you can both pull from and push to.

For example if you called your remote "github", you can verify your
configuration like so:

```sh
$ git remote -v
github  git@github.com:jdoe1234/gluster-csi-driver.git (fetch)
github  git@github.com:jdoe1234/gluster-csi-driver.git (push)
origin  https://gluster/gluster-csi-driver (fetch)
origin  https://gluster/gluster-csi-driver (push)
```

### Building and Testing

To build the gluster-csi-driver, type `make`
from the top of the gluster-csi-driver source tree.

gluster-csi-driver comes with a suite of module and unit tests. To run the suite of
code quality checks, unit tests run `make test` from the top
of the gluster-csi-driver source tree.

## Contributor's Workflow

Here is a guide on how to work on a new patch and get it included
in the official gluster-csi-driver sources.

### Preparatory work

Before you start working on a change, you should check the existing
issues and pull requests for related content. Maybe someone has
already done some analysis or even started a patch for your topic...

### Working on the code and creating patches

In this example, we will work on a patch called *hellopatch*:

1. `git checkout master`
2. `git pull`
3. `git checkout -b hellopatch`

Do your work here and then commit it. For example, run `git commit -as`,
to automatically include all your outstanding changes into a new
patch.

#### Splitting your change into commits

Generally, you will not just commit all your changes into a single
patch but split them up into multiple commits. It is perfectly
okay to have multiple patches in one pull request to achieve
the higher level goal of the pull request. (For example one patch
fix a bug and one patch to add a regression test.)

You can use `git add -i` to select which hunks of your change to
commit. `git rebase -i` can be used to polish up a sequence of
work-in-progress patches into a sequence of patches of merge quality.

gluster-csi-driver's guidelines for the contents of commits are:

- Commits should usually be as minimal and atomic as possible.
- I.e. a patch should only contain one logical change but achieve it completely.
- If the commit does X and Y, you should probably split it into two patches.
- Each patch should compile and pass `make test`

#### Good commit messages

Each commit has a commit message. The gluster-csi-driver project prefers
commit messages roughly of the following form:

```
component(or topic)[:component]: Short description of what the patch does

Optionally longer explanation of the why and how.

Signed-off-by: Author Name <author@email>
```

#### Linking to issues

If you are working on an existing issue you should make sure to use
the appropriate [keywords](https://help.github.com/articles/closing-issues-via-commit-messages/)
in your commit message (e.g. `Fixes #<issue-number>`).
Doing so will allow GitHub to automatically
create references between your changes and the issue.

### Testing the Change

Each pull request needs to pass the basic test suite in order
to qualify for merging. It is hence highly recommended that you
run at least the basic test suite on your branch, preferably
even on each individual commit and make sure it passes
before submitting your changes for review.

#### Basic Tests

As mentioned in the section above gluster-csi-driver has a suite of quality checks and
unit tests that should always be run before submitting a change to the
project. The simplest way to run this suite is to run `make test` in the
top of the source tree.

Sometimes it may not make sense to run the entire suite, especially if you
are iterating on changes in a narrow area of the code. In this case, you
can execute the [Go language test tool](https://golang.org/cmd/go/#hdr-Test_packages)
directly. When using `go test` you can specify a package (sub-directory)
and the tool will only run tests in that directory. For example:

```
go test -v github.com/gluster/gluster-csi-driver/pkg/glusterfs
```

### Pull Requests

Once you are satisfied with your changes you can push them to your gluster-csi-driver fork
on GitHub. For example, `git push github hellopatch` will push the contents
of your hellopatch branch to your fork.

Now that the patch or patches are available on GitHub you can use the GitHub
interface to create a pull request (PR). If you are submitting a single patch
GitHub will automatically populate the PR description with the content of
the change's commit message. Otherwise provide a brief summary of your
changes and complete the PR.

Usually, a PR should concentrate on one topic like a fix for a
bug, or the implementation of a feature. This can be achieved
with multiple commits in the patchset for this PR, but if your
patchset accomplishes multiple independent things, you should
probably split it up and create multiple PRs.

*NOTE*: The PR description is not a replacement for writing good commit messages.
Remember that your commit messages may be needed by someone in the future
who is trying to learn why a particular change was made.

### Patch Review

Now other developers involved in the project will provide feedback on your
PR. If a maintainer decides the changes are good as-is they will merge
the PR into the main gluster-csi-driver repository. However, it is likely that some
discussion will occur and changes may be requested.

### Iterating on Feedback

You will need to return to your local clone and work through the changes
as requested. You may end up with multiple changes across multiple commits.
The gluster-csi-driver project developers prefer a linear history where each change is
a clear logical unit. This means you will generally expect to rebase
your changes.

Run `git rebase -i master` and you will be presented with output something
like the following:

```
pick e405b76 my first change
pick ac78522 my second change
...
pick bf34223 my twentythird change

# Rebase d03eaf9..bf34223 onto d03eaf9
#
# Commands:
#  p, pick = use commit
#  r, reword = use commit, but edit the commit message
#  e, edit = use commit, but stop for amending
#  s, squash = use commit, but meld into previous commit
#  f, fixup = like "squash", but discard this commit's log message
#  x, exec = run command (the rest of the line) using shell
...
```

What you do here is highly dependent on what changes were requested but let's
imagine you need to combine the change "my twentythird change" with
"my first change". In that case you could alter the file such that it
looks like:

```
pick e405b76 my first change
fixup bf34223 my twentythird change
pick ac78522 my second change
...
```

This will instruct git to combine the two changes, throwing away the
latter change's commit message.

Once done you need to re-submit your branch to GitHub. Since you've altered
the existing content of the branch you will likely need to force push,
like so: `git push -f github hellopatch`.

After iterating on the feedback you have got from other developers the
maintainers may accept your patch. It will be merged into the project
and you can delete your branch. Congratulations!
