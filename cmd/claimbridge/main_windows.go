// NGN-written stdio bridge: WSL taskset does not constrain Windows children.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"unsafe"
)

type basicLimits struct {
	ProcessTime, JobTime         int64
	Flags                        uint32
	MinWorkingSet, MaxWorkingSet uintptr
	ActiveProcesses              uint32
	Affinity                     uintptr
	Priority, Scheduling         uint32
}
type ioCounters struct{ ReadOps, WriteOps, OtherOps, ReadBytes, WriteBytes, OtherBytes uint64 }
type extendedLimits struct {
	Basic                                                      basicLimits
	IO                                                         ioCounters
	ProcessMemory, JobMemory, PeakProcessMemory, PeakJobMemory uintptr
}

func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("usage: claimbridge.exe <anchor.exe>")
	}
	path := os.Args[1]
	if strings.HasPrefix(path, "/mnt/c/") {
		path = "C:\\" + strings.ReplaceAll(strings.TrimPrefix(path, "/mnt/c/"), "/", "\\")
	}
	k := syscall.NewLazyDLL("kernel32.dll")
	self, _, _ := k.NewProc("GetCurrentProcess").Call()
	ok, _, e := k.NewProc("SetProcessAffinityMask").Call(self, 0xffff)
	if ok == 0 {
		return fmt.Errorf("bridge affinity: %w", e)
	}
	ok, _, e = k.NewProc("SetPriorityClass").Call(self, 0x40)
	if ok == 0 {
		return fmt.Errorf("bridge priority: %w", e)
	}
	create := k.NewProc("CreateJobObjectW")
	job, _, e := create.Call(0, 0)
	if job == 0 {
		return fmt.Errorf("CreateJobObject: %w", e)
	}
	defer syscall.CloseHandle(syscall.Handle(job))
	limits := extendedLimits{Basic: basicLimits{Flags: 0x2000 | 0x10 | 0x20, Affinity: 0xffff, Priority: 0x40}}
	ok, _, e = k.NewProc("SetInformationJobObject").Call(job, 9, uintptr(unsafe.Pointer(&limits)), unsafe.Sizeof(limits))
	if ok == 0 {
		return fmt.Errorf("job limits: %w", e)
	}
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), "GOMAXPROCS=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x40} // Idle from creation.
	stdin, e := cmd.StdinPipe()
	if e != nil {
		return e
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if e = cmd.Start(); e != nil {
		return e
	}
	fail := func(e error) error { cmd.Process.Kill(); cmd.Wait(); return e }
	process, _, e := k.NewProc("OpenProcess").Call(0x0001|0x0100|0x0200|0x1000|0x00100000, 0, uintptr(cmd.Process.Pid))
	if process == 0 {
		return fail(fmt.Errorf("OpenProcess: %w", e))
	}
	defer syscall.CloseHandle(syscall.Handle(process))
	ok, _, e = k.NewProc("AssignProcessToJobObject").Call(job, process)
	if ok == 0 {
		return fail(fmt.Errorf("AssignProcessToJobObject: %w", e))
	}
	var mask, system uintptr
	ok, _, e = k.NewProc("GetProcessAffinityMask").Call(process, uintptr(unsafe.Pointer(&mask)), uintptr(unsafe.Pointer(&system)))
	if ok == 0 || mask != 0xffff {
		return fail(fmt.Errorf("affinity mask=%x: %v", mask, e))
	}
	priority, _, e := k.NewProc("GetPriorityClass").Call(process)
	if priority != 0x40 {
		return fail(fmt.Errorf("priority=%x: %v", priority, e))
	}
	fmt.Fprintf(os.Stderr, "NGN claimbridge affinity 0xffff priority 0x40 pid %d\n", cmd.Process.Pid)
	// No UCI input reaches the anchor until its job limits are verified.
	go func() { io.Copy(stdin, os.Stdin); stdin.Close() }()
	return cmd.Wait()
}
func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, "fatal claimbridge:", e)
		os.Exit(1)
	}
}
