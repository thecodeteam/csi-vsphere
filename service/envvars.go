package service

// EnvVarPort is the environment variable used to specify the port
// used to connect to the vSphere service. The default value is 1019.
const EnvVarPort = "X_CSI_VSPHERE_PORT"

// EnvVarDatastore is the environment variable that specifies the
// datastore from which VMDKs are listed and the default datastore
// in/from which VMDKs are created/removed.
const EnvVarDatastore = "X_CSI_VSPHERE_DATASTORE"
