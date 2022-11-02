# Scatman

![scatman-icon-stroked](https://user-images.githubusercontent.com/598882/189295890-e50e5281-34dc-4fc4-ae03-401ede382357.png)


Scatman aims to be a **very** simple (as in few features) chroot and image manager.

## Scatman?

Scatman = **Scat**ole **Man**ager

Scatole means Boxes in Italian 🇮🇹 🇮🇹 🇮🇹
So literally a Boxes Manager.

## So what does this manage?

Scatman is a Linux [chroot](https://it.wikipedia.org/wiki/Chroot) manager, with some utilities attached to:

- Download and manager images
- Compatibility with **any OCI registry to pull images**
- Convert an OCI image in a simple rootfs
- **Chroot in it**

All of this is possible thanks to some underlying technologies like:

- Linux's user and mount namespaces
- Linux user and group remapping
- [Rootlesskit](https://github.com/rootless-containers/rootlesskit/) (very very handy tool! Check it out!)

> **Warning**
> This is beta quality software, it's not heavily used and tested like other alternatives. Be aware.

## So are these containers right?

Well... no, not really. Sure you have a separate user, mount (and optionally network, pid, ipc) namespaces, and the processes are in a chroot jail, but this does not manage anything else, so: 

- no seccomp
- no capabilities
- no cgroups

If you need full blown containers, look no further than [Podman](https://github.com/containers/podman) or [Nerdctl](https://github.com/containerd/nerdctl) for your needs.

## Goals

Scatman wants to be:

- nimble 
- single statically compiled binary
- no external dependencies (as much as possible...more on this later)
- Follow Podman command and flags name when possible, try also to match the output

**This tool does not aim to be a replacement for Podman, Docker, Nerdctl or similar tools**

### Ok but why was it created?

Well, I felt the need to go deeper in how to download a container image from a registry... then one thing lead to another... and here we are. 👀

Also this could be (in the future) a nice fallback for [Distrobox](https://github.com/89luca89/distrobox) when no container-manager is found, or when it's not possible to install one.

# Getting started

## Install

Download the binary from the release page, and use it.

## Compile

```console
CGO_ENABLED=0 go build
```

This will create a statically compiled binary.

### Dependencies

By itself Scatman depends only on some Linux utilities (nsenter, tar, cp, ps etc etc), those will be sourced from a bundled `busybox` static binary. This ensures working dependencies even on atypical systems.

Another dependency is [Rootlesskit](https://github.com/rootless-containers/rootlesskit/). This dependency is also managed at runtime (the first time) and is automatically downloaded and used.

But be aware that for Rootlesskit to work, you need to have a working installation of the `uidmap` package.

Citing their README:

### subuid
- `newuidmap` and `newgidmap` need to be installed on the host. These commands are provided by the `uidmap` package on most distributions.

- `/etc/subuid` and `/etc/subgid` should contain more than 65536 sub-IDs. e.g. `penguin:231072:65536`. These files are automatically configured on most distributions.

See also [https://rootlesscontaine.rs/getting-started/common/subuid/](https://rootlesscontaine.rs/getting-started/common/subuid/)

## Usage

Which commands are available:

```console
scatman 
Manage chroots and images

Usage:
  scatman [options] [command]

Available Commands:
  cp          Copy files/folders between a chroot and the local filesystem
  create      Create but do not start a chroot
  exec        Run a process in a running chroot
  help        Help about any command
  images      List images in local storage
  logs        Fetch the logs of one or more chroots
  ps          List chroots
  pull        Pull an image from a registry
  rm          Remove one or more chroots
  rmi         Removes one or more images from local storage
  start       Start one or more chroots
  stop        Stop one or more chroots
  version     Display the scatman version information
```

Pull an image:

```console
:~$ scatman pull registry.fedoraproject.org/fedora-toolbox:36
2022/09/09 01:04:51 Trying to pull registry.fedoraproject.org/fedora-toolbox:36 ...
2022/09/09 01:04:51 Copying config 2110dbbc33d2 done
Copying blob layer0.tar 100% |██████████████████████████████| (5.0 MB/s)         
2022/09/09 01:05:03 Copying blob layer0.tar done
Copying blob layer1.tar 100% |██████████████████████████████| (5.1 MB/s)         
2022/09/09 01:05:15 Copying blob layer1.tar done
2022/09/09 01:05:15 Writing manifest to image destination
2110dbbc33d288a81b2c6f8658130f7c6dc46704d32eb13bb5b30f6e24ee6125
```

Create the first chroot:

```console
:~$ scatman create --name first-scatman docker.io/alpine:latest /bin/sh -l
2022/09/09 00:53:59 Extracting 77bee1403c418adef29535705cb247a88a75753986667c7d0b168e032d7459a0/layer0.tar...
2022/09/09 00:53:59 Extraction complete.
2022/09/09 00:53:59 Created first-scatman successfully.
```

Start the chroot:

```console
:~$ scatman start --interactive first-scatman 
2022/09/09 00:54:32 Starting: first-scatman
first-scatman:/# 
```

Exec a command in an existing chroot:

```console
:~$ scatman exec -i first-scatman cat /etc/os-release
2022/09/09 00:56:23 Entering: first-scatman
NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.16.2
PRETTY_NAME="Alpine Linux v3.16"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"
```

Stop the chroot:

```console
:~$ scatman stop first-scatman
2022/09/09 00:54:58 Stopping first-scatman...
2022/09/09 00:54:58 Killing pid: 115828
2022/09/09 00:54:58 Killing pid: 115823
2022/09/09 00:54:58 Stopped: first-scatman
```

---

For more advanced use, you can always use `--help` to have informations about the commands to launch.

---

# Performance

Doing like 1/20th of what Podman or Nerdctl do, at least it tries to be fast...

There are some basic entering speed for an execution:

```console
:~$ time (for i in {1..20}; do podman exec -ti --user luca-linux fedora-rawhide whoami >/dev/null 2>/dev/null; done)

real    0m4.690s
user    0m2.178s
sys     0m0.829s
:~$ time (for i in {1..20}; do ./scatman exec -i --user luca-linux fedora-rawhide whoami >/dev/null 2>/dev/null; done)

real    0m1.190s
user    0m0.420s
sys     0m0.869s
```

**It takes about 50~60ms to enter a chroot and execute stuff**

This obviously is a completely useless and arbitrary metric compared to the difference of utility of the two tools.

# Limitations

For now:

- `pull` does not have [shortnames](https://github.com/containers/shortnames) support (yet)
- by nature this tool does not use stuff come `overlayfs` so **there is no deduplication**

# TO DO

- Write tests, tests, tests tests!
- Did I mention more tests??
- Documentation of course
- Create manpages from the usage docs automatically
- Configurable paths (for now it's ~/.local/share/scatman)
- `logs --since` flag is very useful
- an `inspect` command would be useful as well
- a `run` command would be useful as well
- automatically `pull` when `create` if image does not exist

---

Extras:

```
:~$ scatman sing

____ _  _ _    ___  _  ___  _ ___  ___  _   _  ___  _ ___
[__  |_/  | __ |__] |  |  \ | |__] |__]  \_/   |  \ | |__]
___] | \_ |    |__] |  |__/ | |__] |__]   |    |__/ | |__]
_   _____  ___  ____  ___  _  _ ___   ___  _  _ ___
 \_/ |  |  |  \ |__|  |  \ |  | |__]  |  \ |  | |__]
  |  |__|  |__/ |  |  |__/ |__| |__]  |__/ |__| |__]

    .~7~.              .!P&@@@@@@&P~    .?G&@@@@@@&#5^             .?B&B?.
  ^#@@57!^           ~G@@@@@@@@@@@@@&?~G@@@@@@@@@@@@@@&5:         .7^^J@@@?
 !@@@:            .Y&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@B~            .&@@Y
:@@@Y           ~G@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#!           J@@@!
P@@@?        :Y&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@&?.        7@@@#
#@@@B     .7#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@P^     :&@@@&
G@@@@#J?5#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@BPG#@@@@@#
~@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@!
 B@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#^ 7&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@5
 .#@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@#~     ?&@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@G
   5@@@@@@@@@@@@@@@@@@@@@@@@@@@#J.         ^5&@@@@@@@@@@@@@@@@@@@@@@@@@@&7
    :Y&@@@@@@@@@@@@@@@@@@@@BJ~.               .!P#@@@@@@@@@@@@@@@@@@@@B~
       .~YB&@@@@@@@&#BP?~:                        .:~JPB#&&&@@@&&#P7^.

```
