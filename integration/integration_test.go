package integration

import (
	"strings"
	"testing"

	"github.com/u-root/u-root/pkg/qemu"
)

func testWithQEMU(t *testing.T, init string) *qemu.QEMU {
	// TODO: check if QEMU variable is set.

	q := &qemu.QEMU{
		Init: init,
		Kernel: "testdata/bzImage_amd64", // TODO: select a better kernel
		InitRamfs: "/tmp/initramfs.linux_amd64.cpio", // TODO: build on-the-fly
	}
	t.Log("command line: '" + strings.Join(q.CmdLine(), "' '") + "'")
	if err := q.Start(); err != nil {
		t.Fatal("could not spawn QEMU: ", err)
	}
	return q
}

// TestHelloWorld runs an init which prints the string "HELLO WORLD" and exits.
func TestHelloWorld(t *testing.T) {
	// Create the CPIO and start QEMU.
	q := testWithQEMU(t, "testdata/helloworld.go")
	defer q.Close()

	if err := q.Expect("NR_IRQS"); err != nil {
		t.Fatal(err)
	}
}

func TestKexec(t *testing.T) {
	// Create the CPIO and start QEMU.
	q := testWithQEMU(t, "testdata/helloworld.go")
	defer q.Close()

	if err := q.Expect("NR_IRQS"); err != nil {
		t.Fatal(err)
	}
}

// TestGoTests runs `go test ./...` inside of QEMU. This allows tests requiring
// root priviledges to run.
func TestGoTests(t *testing.T) {
	// Create the CPIO and start QEMU.
	q := testWithQEMU(t, "testdata/helloworld.go")
	defer q.Close()

	if err := q.Expect("NR_IRQS"); err != nil {
		t.Fatal(err)
	}
}
