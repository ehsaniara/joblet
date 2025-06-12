package main

import (
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"runtime"
	"syscall"
)

func main() {
	fmt.Printf("=== Go 1.24 Namespace Capability Test ===\n\n")

	// Test 1: Basic system info
	fmt.Printf("Go Version: %s\n", runtime.Version())
	fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Running as UID: %d\n\n", os.Getuid())

	// Test 2: Check SysProcAttr fields
	testSysProcAttrFields()

	// Test 3: Check unshare availability
	testUnshareAvailability()

	// Test 4: Test actual namespace creation
	testNamespaceCreation()

	// Test 5: Recommendation
	giveRecommendation()
}

func testSysProcAttrFields() {
	fmt.Println("=== SysProcAttr Field Analysis ===")

	sysProcAttrType := reflect.TypeOf(syscall.SysProcAttr{})
	fmt.Printf("SysProcAttr has %d fields:\n", sysProcAttrType.NumField())

	hasCloneflags := false
	for i := 0; i < sysProcAttrType.NumField(); i++ {
		field := sysProcAttrType.Field(i)
		fmt.Printf("  %s: %s (offset: %d)\n", field.Name, field.Type, field.Offset)

		if field.Name == "Cloneflags" {
			hasCloneflags = true
		}
	}

	if hasCloneflags {
		fmt.Println("✅ Cloneflags field found!")
	} else {
		fmt.Println("❌ Cloneflags field NOT found")
	}
	fmt.Println()
}

func testUnshareAvailability() {
	fmt.Println("=== unshare Command Test ===")

	// Test if unshare exists
	cmd := exec.Command("which", "unshare")
	if output, err := cmd.Output(); err != nil {
		fmt.Println("❌ unshare command not found")
	} else {
		fmt.Printf("✅ unshare found at: %s", string(output))
	}

	// Test unshare help
	cmd = exec.Command("unshare", "--help")
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ unshare --help failed: %v\n", err)
	} else {
		fmt.Println("✅ unshare --help works")
	}

	// Test specific namespace flags
	namespaces := []string{"--pid", "--mount", "--ipc", "--uts", "--net"}
	for _, ns := range namespaces {
		cmd = exec.Command("unshare", ns, "--", "true")
		if err := cmd.Run(); err != nil {
			fmt.Printf("❌ %s namespace: %v\n", ns, err)
		} else {
			fmt.Printf("✅ %s namespace works\n", ns)
		}
	}
	fmt.Println()
}

func testNamespaceCreation() {
	fmt.Println("=== Namespace Creation Test ===")

	// Test 1: Simple PID namespace
	fmt.Print("Testing PID namespace: ")
	cmd := exec.Command("unshare", "--pid", "--fork", "--", "echo", "test")
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Success")
	}

	// Test 2: Multiple namespaces
	fmt.Print("Testing multiple namespaces: ")
	cmd = exec.Command("unshare", "--pid", "--mount", "--ipc", "--uts", "--fork", "--", "echo", "test")
	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Success")
	}

	// Test 3: Process isolation verification
	fmt.Print("Testing process isolation: ")
	cmd = exec.Command("unshare", "--pid", "--fork", "--", "sh", "-c", "ps aux | wc -l")
	if output, err := cmd.Output(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Printf("✅ Success (saw %s processes)\n", string(output)[:len(output)-1])
	}
	fmt.Println()
}

func giveRecommendation() {
	fmt.Println("=== Recommendation for Go 1.24 ===")

	sysProcAttrType := reflect.TypeOf(syscall.SysProcAttr{})
	_, hasCloneflags := sysProcAttrType.FieldByName("Cloneflags")

	cmd := exec.Command("unshare", "--help")
	hasUnshare := cmd.Run() == nil

	testCmd := exec.Command("unshare", "--pid", "--fork", "--", "true")
	canCreateNS := testCmd.Run() == nil

	fmt.Printf("System Assessment:\n")
	fmt.Printf("  Cloneflags available: %v\n", hasCloneflags)
	fmt.Printf("  unshare available: %v\n", hasUnshare)
	fmt.Printf("  Can create namespaces: %v\n", canCreateNS)
	fmt.Printf("  Running as root: %v\n\n", os.Getuid() == 0)

	if hasCloneflags {
		fmt.Println("🎉 RECOMMENDED: Use Cloneflags approach")
		fmt.Println("   Your Go 1.24 build has Cloneflags support!")
		printCloneflagsExample()
	} else if hasUnshare && canCreateNS {
		fmt.Println("👍 RECOMMENDED: Use unshare approach")
		fmt.Println("   Cloneflags not available, but unshare works well")
		printUnshareExample()
	} else if hasUnshare {
		fmt.Println("⚠️  LIMITED: unshare available but needs permissions")
		fmt.Println("   Try running as root or with proper capabilities")
		printPermissionHelp()
	} else {
		fmt.Println("❌ NO NAMESPACE SUPPORT")
		fmt.Println("   Install util-linux package for unshare")
		printInstallHelp()
	}
}

func printCloneflagsExample() {
	fmt.Println(`
Example implementation:
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWPID | syscall.CLONE_NEWNS | 
		           syscall.CLONE_NEWIPC | syscall.CLONE_NEWUTS,
		Setpgid: true,
		Setsid:  true,
	}`)
}

func printUnshareExample() {
	fmt.Println(`
Example implementation:
	args := []string{"--pid", "--mount", "--ipc", "--uts", "--fork", "--", command}
	args = append(args, originalArgs...)
	cmd := exec.Command("unshare", args...)`)
}

func printPermissionHelp() {
	fmt.Println(`
Permission solutions:
1. Run as root: sudo ./your-program
2. Set capabilities: sudo setcap cap_sys_admin+ep ./your-program
3. Use Docker with --privileged flag
4. Configure systemd service with proper capabilities`)
}

func printInstallHelp() {
	fmt.Println(`
Install unshare:
  Ubuntu/Debian: sudo apt-get install util-linux
  CentOS/RHEL:   sudo yum install util-linux
  Alpine:        sudo apk add util-linux`)
}

// Simple test to verify your current setup works
func testCurrentSetup() {
	fmt.Println("\n=== Quick Setup Test ===")

	// Test what would actually work in your job worker
	testBasicExecution()
	testUnshareExecution()
}

func testBasicExecution() {
	fmt.Print("Basic command execution: ")
	cmd := exec.Command("echo", "hello")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Setsid:  true,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Works")
	}
}

func testUnshareExecution() {
	fmt.Print("unshare execution: ")
	cmd := exec.Command("unshare", "--pid", "--fork", "--", "echo", "hello")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Setsid:  true,
	}

	if err := cmd.Run(); err != nil {
		fmt.Printf("❌ Failed: %v\n", err)
	} else {
		fmt.Println("✅ Works")
	}
}
