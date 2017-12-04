package main

import (
	"context"

	"github.com/thecodeteam/gocsi/csp"

	"github.com/thecodeteam/csi-vsphere/provider"
	"github.com/thecodeteam/csi-vsphere/service"
)

// main is ignored when this package is built as a go plug-in.
func main() {
	csp.Run(
		context.Background(),
		service.Name,
		"A description of the SP",
		"",
		provider.New())
}

const usage = `    X_CSI_VSPHERE_PORT
        The port used to connect to the ESX service.

        The default value is 1019.

    X_CSI_VSPHERE_DATASTORE
        The datastore from which VMDKs are listed and the default
        datastore in/from which VMDKs are created/removed.
`
