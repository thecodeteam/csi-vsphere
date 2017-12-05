# CSI Storage Plug-in (SP) for VMware vSphere
CSI-vSphere is a Container Storage Interface
([CSI](https://github.com/container-storage-interface/spec)) plug-in
that that provides VMDK support to vSphere virtual machine (VM).

## Runtime Dependencies
The CSI-vSphere SP has the following requirements:
1. The SP supports the CSI [decentralized model](https://github.com/container-storage-interface/spec/blob/master/spec.md#architecture) and must be deployed to all VMs that require access to storage.
2. A VM must be located on a host running the
[ESXi component](http://vmware.github.io/docker-volume-vsphere/documentation/prerequisite.html#vsphere-installation-bundle-vib)
of the VMware vSphere Docker Volume Service ([vDVS](http://vmware.github.io/docker-volume-vsphere/documentation/index.html)).
The SP's RPCs `ControllerProbe` and `NodeProbe` will only be successful if the VM is able to access the backend, ESXi component.

The following command may be used to verify a VM can access the ESXi vDVS component:

```bash
$ docker run -it golang sh -c \
  "go get github.com/thecodeteam/csi-vsphere/cmd/vmdkops && vmdkops"
```

The above command will complete without error if the VM is able to successfully
communicate with the service running on the ESXi host.

## Installation
CSI-vSphere may be installed with the following command:

```bash
$ go get github.com/thecodeteam/csi-vsphere
```

The resulting binary is located at `$GOPATH/bin/csi-vsphere`.

## Starting the Plug-in
Before starting the plug-in please set the environment variable
`CSI_ENDPOINT` to a valid Go network address such as `csi.sock`:

```bash
$ CSI_ENDPOINT=csi.sock csi-vsphere
INFO[0000] serving                                       address="unix://csi.sock"
```

The server can be shutdown by using `Ctrl-C` or sending the process
any of the standard exit signals.

## Using the Plug-in
The CSI specification uses the gRPC protocol for plug-in communication.
The easiest way to interact with a CSI plug-in is via the Container
Storage Client (`csc`) program provided via the
[GoCSI](https://github.com/thecodeteam/gocsi) project:

```bash
$ go get github.com/thecodeteam/gocsi/csc
```

## Configuration
The CSI-vShere SP is configured via environment variables:

| Name | Default | Description |
|------|---------|-------------|
| `X_CSI_VSPHERE_PORT` | `1019` | The port used to connect to the ESX service |
| `X_CSI_VSPHERE_DATASTORE` | | The datastore from which VMDKs are listed and the default datastore in/from which VMDKs are created/removed. |

This SP is built using the
[GoCSI CSP package](https://github.com/thecodeteam/gocsi/tree/master/csp)
and as such may be configured with any of its
[configuration properties](https://github.com/thecodeteam/gocsi/tree/master/csp#configuration).
The following table is a list of the global configuration properties for
which CSI-vSphere provides a default value:

| Name | Value | Description |
|------|---------|-----------|
| `X_CSI_IDEMP` | `true` | Enables idempotency |
| `X_CSI_IDEMP_REQUIRE_VOL` | `true` | Instructs the idempotency interceptor to validate the existence of a volume before allowing an operation to proceed |
| `X_CSI_CREATE_VOL_ALREADY_EXISTS` | `true` | Indicates that a `CreateVolume` request with a result of `AlreadyExists` will be changed to success |
| `X_CSI_DELETE_VOL_NOT_FOUND` | `true` | Indicates that a `DeleteVolume` request with a result of `NotFound` will be changed to success |
| `X_CSI_SUPPORTED_VERSIONS` | `0.0.0, 0.1.0` | A list of the CSI versions this SP supports |

## Access Modes
The CSI-vSphere SP supports the following CSI volume
[access modes](https://github.com/container-storage-interface/spec/blob/master/spec.md#createvolume):

| Access Mode | Description |
|-------------|-------------|
| `SINGLE_NODE_WRITER` | Can only be published once as read/write on a single node, at any given time. |
| `SINGLE_NODE_READER_ONLY` | Can only be published once as readonly on a single node, at any given time. |

## Support
For any questions or concerns please file an issue with the
[CSI-vSphere](https://github.com/thecodeteam/csi-vsphere/issues) project or join
the Slack channel #project-rexray at codecommunity.slack.com.
