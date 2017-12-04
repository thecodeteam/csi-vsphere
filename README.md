# CSI-vSphere
A Container Storage Interface (CSI) Storage Plug-in (SP) for VMware vSphere.

```shell
$ CSI_ENDPOINT=csi.sock \
  X_CSI_LOG_LEVEL=info \
  ./csi-vsphere
INFO[0000] serving                                       endpoint="unix://csi.sock"
```

## Configuration
This CSI plug-in was created using the GoCSI
[`csp`](https://github.com/thecodeteam/gocsi/tree/master/csp) package.
Please see its [configuration section](https://github.com/thecodeteam/gocsi/tree/master/csp/README.md#configuration)
for a complete list of the environment variables that may be used to configure
this SP.
