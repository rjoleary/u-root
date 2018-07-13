package integration

import (
	"testing"

	"github.com/u-root/u-root/pkg/qemu"
)

func testWithQEMU(t *testing.T, init string) *qemu.QEMU {
	if _, err := os.Lookup("UROOT_QEMU"); err != nil {
		t.Skip("test is skipped unless UROOT_QEMU is set")
	}

	// Create a temporary directory.
	tmpDir, err := Tempdir("", "uroot-integration")
	if err != nil {
		t.Fatal(err)
	}

	// Build init.
	cmd := exec.Command("go", "build", init)
	cmd.Path = tmpDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// Place init into cpio.
	// TODO: use u-root cpio API
	cmd = exec.Command("cpio", "-ov", "-h", "newc")
	cmd.Path = tmpDir
	cmd.Stdin = 
	cmd.Stdout = os.Stdout

	// Build u-root with the given inito.

	q := &qemu.QEMU{
		InitRamfs: "/tmp/initramfs.linux_amd64.cpio", // TODO: build on-the-fly
		Kernel: "testdata/bzImage_amd64", // TODO: select a better kernel
	}
	t.Logf("command line:\n%s", q.CmdLineQuoted())
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

// TestWgetKexec runs an init which will first download a kernel and initramfs,
// then kexecs it (without validation).
func TestWgetKexec(t *testing.T) {
	// Create the CPIO and start QEMU.
	q := testWithQEMU(t, "testdata/helloworld.go")
	defer q.Close()

	// TODO
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

	// TODO
	if err := q.Expect("NR_IRQS"); err != nil {
		t.Fatal(err)
	}
}
