package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/thecodeteam/csi-vsphere/pkg/vmware/vmdkops"
)

func main() {

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr,
			"usage: %s [CMD] [VMDK] [KEY=VAL [KEY=VAL]...]", os.Args[0])
		os.Exit(1)
	}
	port := flag.Int(
		"port", 1019, "The port used to connect to the ESXi service.")
	flag.Parse()

	if port == nil {
		vmdkops.EsxPort = 1019
	} else {
		vmdkops.EsxPort = *port
	}

	var (
		cmdName string
		dskName string
		opts    map[string]string
	)

	if len(os.Args) < 2 {
		cmdName = "list"
	} else {
		cmdName = os.Args[1]
	}

	if len(os.Args) > 2 {
		dskName = os.Args[2]
	}

	if len(os.Args) > 3 {
		opts = map[string]string{}
		for _, a := range os.Args[3:] {
			p := strings.SplitN(a, "=", 2)
			if len(p) == 0 {
				continue
			}
			k := p[0]
			var v string
			if len(p) > 1 {
				v = p[1]
			}
			opts[k] = v
		}
	}

	cmd := vmdkops.EsxVmdkCmd{Mtx: &sync.Mutex{}}
	out, err := cmd.Run(cmdName, dskName, opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	if _, err := os.Stdout.Write(out); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println()
}
