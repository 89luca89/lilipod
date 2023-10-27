# Lilipod

<picture>
<source srcset="assets/logo-dark.svg" media="(prefers-color-scheme: dark)">
<img src="assets/logo-light.svg">
</picture>

Lilipod aims to be a **very** simple (as in few features) container and image manager.

## Lilipod?

Sounds like little-pod, and I like it.

## So what does this manage?

Lilipod is a very simple container manager with minimal features to:

- Download and manager images
- Create and run containers 

It tries to keep a **somewhat** compatible CLI interface with Podman/Docker/Nerdctl

```shell
~$ lilipod help
Manage containers and images

Usage:
  lilipod [command]

Available Commands:
  completion      Generate the autocompletion script for the specified shell
  cp              Copy files/folders between a container and the local filesystem
  create          Create but do not start a container
  exec            Exec but do not start a container
  help            Help about any command
  images          List images in local storage
  inspect         Inspect a container or image
  logs            Fetch the logs of one or more 
  ps              List containers
  pull            Pull an image from a registry
  rename          Rename a container
  rm              Remove one or more containers
  rmi             Removes one or more images from local storage
  run             Run but do not start a container
  start           Start one or more containers
  stop            Remove one or more containers
  update          Update but do not start a container
  version         Show lilipod version

Flags:
  -h, --help               help for lilipod
      --log-level string   log messages above specified level (debug, warn, warning, error)
  -v, --version            version for lilipod

Use "lilipod [command] --help" for more information about a command.

```

> **Warning**
> This is beta quality software, it's not heavily used and tested like other alternatives. Be aware.

## So are these containers like in Podman or Docker right?

![image](https://github.com/89luca89/scatman/assets/598882/d01d62d6-f659-4c07-b474-cd6505ff8ab5)

Well...superficially yes; Sure you have a separate user, mount (and optionally network, pid, ipc) namespaces, and the processes are in a pivotroot jail, but this does not manage anything else, so: 

- no seccomp
- no capabilities
- no cgroups

If you need full blown containers, look no further than [Podman](https://github.com/containers/podman) or [Nerdctl](https://github.com/containerd/nerdctl) for your needs.

## Goals

Lilipod wants to be:

- nimble 
- single statically compiled binary
- no external dependencies (as much as possible...more on this later)
- Follow Podman command and flags name when possible, try also to match the output

**This tool does not aim to be a full replacement for Podman, Docker, Nerdctl or similar tools**

### Ok but why was it created?

Well, I felt the need to go deeper in how to download a container image from a registry... then one thing lead to another... and here we are. 👀

Also this is a nice fallback for [Distrobox](https://github.com/89luca89/distrobox) when no container-manager is found, or when it's not possible to install one.  
Bein a fully self-contained binary that lives in one directory (`LILIPOD_HOME`) it makes it easy to install and remove without package managers.

# Getting started

## Install

Download the binary from the release page, and use it.

## Compile

```console
make
```

This will create a statically compiled binary.

### Dependencies

By itself Lilipod depends only on some Linux utilities (nsenter, tar, cp, ps etc etc), those will be sourced from a bundled `busybox` static binary. This ensures working dependencies even on atypical systems.

But be aware that to work in a rootless manner, you need to have a working installation of the `uidmap` package.

Citing their README:

### subuid
- `newuidmap` and `newgidmap` need to be installed on the host. These commands are provided by the `uidmap` package on most distributions.

- `/etc/subuid` and `/etc/subgid` should contain more than 65536 sub-IDs. e.g. `penguin:231072:65536`. These files are automatically configured on most distributions.

See also [https://rootlesscontaine.rs/getting-started/common/subuid/](https://rootlesscontaine.rs/getting-started/common/subuid/)

## Usage

Which commands are available:

```console
~$ lilipod
Manage containers and images

Usage:
  lilipod [command]

Available Commands:
  completion      Generate the autocompletion script for the specified shell
  cp              Copy files/folders between a container and the local filesystem
  create          Create but do not start a container
  exec            Exec but do not start a container
  help            Help about any command
  images          List images in local storage
  inspect         Inspect a container or image
  logs            Fetch the logs of one or more 
  ps              List containers
  pull            Pull an image from a registry
  rename          Rename a container
  rm              Remove one or more containers
  rmi             Removes one or more images from local storage
  run             Run but do not start a container
  start           Start one or more containers
  stop            Remove one or more containers
  update          Update but do not start a container
  version         Show lilipod version

Flags:
  -h, --help               help for lilipod
      --log-level string   log messages above specified level (debug, warn, warning, error)
  -v, --version            version for lilipod

Use "lilipod [command] --help" for more information about a command.
```

Pull an image:

```console
:~$ lilipod pull registry.opensuse.org/opensuse/tumbleweed:latest
pulling image manifest: registry.opensuse.org/opensuse/tumbleweed:latest
pulling layer db709715e7606a81da9764311fad42de7cebf7cedc14656853797864c5fc5aae.tar.gz
Copying blob sha256:db709715e7606a81da9764311fad42de7cebf7cedc14656853797864c5fc5aae 100% |██████████████████████████████| (4.9 MB/s)         
saving layer sha256:db709715e7606a81da9764311fad42de7cebf7cedc14656853797864c5fc5aae done
saving manifest for registry.opensuse.org/opensuse/tumbleweed:latest
saving config for registry.opensuse.org/opensuse/tumbleweed:latest
saving metadata for registry.opensuse.org/opensuse/tumbleweed:latest
done
84cfef9d6263a008a2d77f4a0863660f
```

Run a container and remove it afterwards:

```console
:~$ lilipod run --rm -ti alpine cat /etc/os-release
NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.18.3
PRETTY_NAME="Alpine Linux v3.18"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"
```

Create the first container:

```console
:~$ lilipod create --name first-lilipod docker.io/alpine:latest /bin/sh -l
f1c35f7b7de161116abb3157bd125f06
```

Start the container:

```console
:~$ lilipod start -ti first-lilipod
first-lilipod:/# 
```

Exec a command in an existing container:

```console
:~$ lilipod exec -ti first-lilipod cat /etc/os-release
NAME="Alpine Linux"
ID=alpine
VERSION_ID=3.18.3
PRETTY_NAME="Alpine Linux v3.18"
HOME_URL="https://alpinelinux.org/"
BUG_REPORT_URL="https://gitlab.alpinelinux.org/alpine/aports/-/issues"
```

Stop the container:

```console
:~$ lilipod stop first-lilipod
first-lilipod
```

Inspect the container:

```console
:~$ lilipod inspect --type container first-lilipod
{
  "env": [
   "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
   "HOSTNAME=first-lilipod",
   "TERM=xterm"
  ],
  "cgroup": "private",
  "created": "2023.09.07 10:17:04",
  "gidmap": "1000:100000:65536",
  "hostname": "first-lilipod",
  "id": "f1c35f7b7de161116abb3157bd125f06",
  "image": "docker.io/alpine:latest",
  "ipc": "private",
  "names": "first-lilipod",
  "network": "private",
  "pid": "private",
  "privileged": false,
  "size": "",
  "status": "stopped",
  "time": "private",
  "uidmap": "1000:100000:65536",
  "user": "root:root",
  "userns": "keep-id",
  "workdir": "/",
  "stopsignal": "SIGTERM",
  "mounts": [],
  "labels": [],
  "entrypoint": [
   "/bin/sh",
   "-l"
  ]
 }
```

Inspect the image:

```console
:-$ lilipod inspect --type image alpine:latest
{
  "architecture": "amd64",
  "container": "ba09fe2c8f99faad95871d467a22c96f4bc8166bd01ce0a7c28dd5472697bfd1",
  "created": "2023-08-07T19:20:20.894140623Z",
  "docker_version": "20.10.23",
  "history": [
   {
    "created": "2023-08-07T19:20:20.71894984Z",
    "created_by": "/bin/sh -c #(nop) ADD file:32ff5e7a78b890996ee4681cc0a26185d3e9acdb4eb1e2aaccb2411f922fed6b in / "
   },
   {
    "created": "2023-08-07T19:20:20.894140623Z",
    "created_by": "/bin/sh -c #(nop)  CMD [\"/bin/sh\"]",
    "empty_layer": true
   }
  ],
  "os": "linux",
  "rootfs": {
   "type": "layers",
   "diff_ids": [
    "sha256:4693057ce2364720d39e57e85a5b8e0bd9ac3573716237736d6470ec5b7b7230"
   ]
  },
  "config": {
   "Cmd": [
    "/bin/sh"
   ],
   "Env": [
    "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
   ],
   "Image": "sha256:39dfd593e04b939e16d3a426af525cad29b8fc7410b06f4dbad8528b45e1e5a9"
  },
  "container_config": {
   "Cmd": [
    "/bin/sh",
    "-c",
    "#(nop) ",
    "CMD [\"/bin/sh\"]"
   ],
   "Env": [
    "PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
   ],
   "Hostname": "ba09fe2c8f99",
   "Image": "sha256:39dfd593e04b939e16d3a426af525cad29b8fc7410b06f4dbad8528b45e1e5a9"
  }
} 
```

Delete the container:

```console
:-$ lilipod rm first-lilipod
first-lilipod
```

---

For more advanced use, you can always use `--help` to have information about the commands to launch.

You can always set the log level to `warn` `error` or `debug` by using the `--log-level` flag:

```console
:-$ lilipod --log-level debug pull alpine:latest
pulling image manifest: index.docker.io/library/alpine:latest
pulling layer 7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de.tar.gz
Copying blob sha256:7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de 100% |██████████████████████████████| (4.9 MB/s)        
saving layer sha256:7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de done
file_utils.go:119 [debug] input checksum is: sha256:7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de
file_utils.go:120 [debug] expected checksum is: sha256:7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de
image_utils.go:354 [debug] successfully checked layer: 7264a8db6415046d36d16ba98b79778e18accee6ffa71850405994cffa9be7de.tar.gz
image_utils.go:116 [debug] 1 layers successfully saved
image_utils.go:117 [debug] cleaning up unwanded files
saving manifest for index.docker.io/library/alpine:latest
saving config for index.docker.io/library/alpine:latest
saving metadata for index.docker.io/library/alpine:latest
done
ff727edbcbe60df2bd6a89cf65d6db2b
```

---

# Performance

Doing like 1/20th of what Podman or Nerdctl do, at least it tries to be fast...

There are some basic entering speed for an execution:

```console
:~$ time (for i in {1..20}; do podman exec -ti --user luca-linux fedora-rawhide whoami >/dev/null 2>/dev/null; done)

real    0m4.690s
user    0m2.178s
sys     0m0.829s
:~$ time (for i in {1..20}; do ./lilipod exec -i --user luca-linux fedora-rawhide whoami >/dev/null 2>/dev/null; done)


real    0m0.741s
user    0m0.458s
sys     0m0.450s

```

**It takes about 5~8ms to enter a container and execute stuff**

This obviously is a completely useless and arbitrary metric compared to the difference of utility of the two tools.

# Configuration

You can set `LILIPOD_HOME` to force lilipod to create images/containers/volumes in a specific directory.

Else lilipod will use `XDG_DATA_HOME` or fallback to `$HOME/.local/share/lilipod`

# Limitations

- by nature this tool does not use stuff like `overlayfs` so **there is no deduplication between container's rootfs**, but **image layer deduplication is present**
- There is no custom networking, you either share host's network or you're offline


# TO DO

- Tests
- Documentation
- Create manpages from the usage docs automatically
- Support Cgroups (low prio)
- Support Capabilities (low prio)
- Support private network (`slirp4netns` probably)
