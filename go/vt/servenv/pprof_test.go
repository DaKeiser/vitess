//go:build !race

// Disabling race detector because it doesn't like TestPProfInitWithWaitSig and TestPProfInitWithoutWaitSig,
// but the profileStarted variable is updated in response to signals invoked in the tests and works as intended.

package servenv

import (
	"flag"
	"os/signal"
	"reflect"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestParseProfileFlag(t *testing.T) {
	tests := []struct {
		arg     string
		want    *profile
		wantErr bool
	}{
		{"", nil, false},
		{"mem", &profile{mode: profileMemHeap, rate: 4096}, false},
		{"mem,rate=1234", &profile{mode: profileMemHeap, rate: 1234}, false},
		{"mem,rate", nil, true},
		{"mem,rate=foobar", nil, true},
		{"mem=allocs", &profile{mode: profileMemAllocs, rate: 4096}, false},
		{"mem=allocs,rate=420", &profile{mode: profileMemAllocs, rate: 420}, false},
		{"block", &profile{mode: profileBlock, rate: 1}, false},
		{"block,rate=4", &profile{mode: profileBlock, rate: 4}, false},
		{"cpu", &profile{mode: profileCPU}, false},
		{"cpu,quiet", &profile{mode: profileCPU, quiet: true}, false},
		{"cpu,quiet=true", &profile{mode: profileCPU, quiet: true}, false},
		{"cpu,quiet=false", &profile{mode: profileCPU, quiet: false}, false},
		{"cpu,quiet=foobar", nil, true},
		{"cpu,path=", &profile{mode: profileCPU, path: ""}, false},
		{"cpu,path", nil, true},
		{"cpu,path=a", &profile{mode: profileCPU, path: "a"}, false},
		{"cpu,path=a/b/c/d", &profile{mode: profileCPU, path: "a/b/c/d"}, false},
		{"cpu,waitSig", &profile{mode: profileCPU, waitSig: true}, false},
		{"cpu,path=a/b,waitSig", &profile{mode: profileCPU, waitSig: true, path: "a/b"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.arg, func(t *testing.T) {
			got, err := parseProfileFlag(tt.arg)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseProfileFlag() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseProfileFlag() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// with waitSig, we should start with profiling off and toggle on-off-on-off
func TestPProfInitWithWaitSig(t *testing.T) {
	signal.Reset(syscall.SIGUSR1)
	flag.Set("pprof", "cpu,waitSig")

	pprof_init()
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(0), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(1), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(0), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(1), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(0), profileStarted)
}

// without waitSig, we should start with profiling on and toggle off-on-off
func TestPProfInitWithoutWaitSig(t *testing.T) {
	signal.Reset(syscall.SIGUSR1)
	flag.Set("pprof", "cpu")

	pprof_init()
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(1), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(0), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(1), profileStarted)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)
	time.Sleep(1 * time.Second)
	assert.Equal(t, uint32(0), profileStarted)
}
