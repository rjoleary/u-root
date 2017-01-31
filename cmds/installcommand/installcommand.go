// Copyright 2012-2017 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Install command from a go source file.
//
// Synopsis:
//     installcommand [-v] [-ludicrous]
//
// Description:
//     u-root commands are lazily compiled. Uncompiled commands in the /bin
//     directory are symbolic links to installcommand. When executed through
//     the symbolic link, installcommand will build the command from source and
//     exec it.
//
// Options:
//     -v: print all build commands
//     -ludicrous: print out ALL the output from the go build commands
import (
	"flag"
	"log"
	"os"
	"os/exec"
	"path"

	"github.com/u-root/u-root/uroot"
)

var (
	urpath    = "/go/bin:/ubin:/buildbin:/usr/local/bin:"
	verbose   = flag.Bool("v", false, "print all build commands")
	ludicrous = flag.Bool("ludicrous", false, "print out ALL the output from the go build commands")
	debug     = func(string, ...interface{}) {}
)

func main() {
	a := []string{"install"}
	/* e.g. (GOBIN=`pwd`/ubin go install uroot.CmdsPath/date) */
	flag.Parse()
	if *verbose {
		debug = log.Printf
		a = append(a, "-x")
	}

	cleanPath := path.Clean(os.Args[0])
	debug("cleanPath %v\n", cleanPath)
	binDir, commandName := path.Split(cleanPath)
	debug("bindir, commandname %v %v\n", binDir, commandName)
	destDir := "/ubin"
	destFile := path.Join(destDir, commandName)

	cmd := exec.Command("go", append(a, path.Join(uroot.CmdsPath, commandName))...)

	// Set GOGC if unset. The best value is determined empirically and
	// depends on the machine and Go version. For the workload of compiling
	// a small Go program, values larger than the default perform better.
	// See: /scripts/build_perf.sh
	if _, ok := os.LookupEnv("GOGC"); !ok {
		cmd.Env = append(os.Environ(), "GOGC=400")
	}

	cmd.Dir = "/"

	debug("Run %v", cmd)
	out, err := cmd.CombinedOutput()
	debug("installcommand: go build returned")

	if err != nil {
		p := os.Getenv("PATH")
		log.Fatalf("installcommand: trying to build cleanPath: %v, PATH %s, err %v, out %s", cleanPath, p, err, out)
	}

	if *ludicrous {
		debug(string(out))
	}

	cmd = exec.Command(destFile)

	cmd.Args = append([]string{commandName}, os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatal(err)
	}
}
