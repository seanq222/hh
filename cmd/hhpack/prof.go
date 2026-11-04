package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
)

var (
	cpuProfile   = flag.String("cpuprofile", "", "write a CPU profile to this file")
	blockProfile = flag.String("blockprofile", "", "write a profile of where goroutines wait on channels and locks to this file")
	mutexProfile = flag.String("mutexprofile", "", "write a profile of lock contention to this file")
)

// startProfiles starts the profiles asked for, and returns the function that
// stops them, writes them out and prints how much memory the run took.
func startProfiles() func() {
	if *cpuProfile != "" {
		f, err := os.Create(*cpuProfile)
		if err == nil {
			err = pprof.StartCPUProfile(f)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
	if *blockProfile != "" {
		runtime.SetBlockProfileRate(1_000_000) // one sample per millisecond blocked
	}
	if *mutexProfile != "" {
		runtime.SetMutexProfileFraction(100)
	}
	return func() {
		if *cpuProfile != "" {
			pprof.StopCPUProfile()
		}
		for _, p := range []struct{ name, path string }{{"block", *blockProfile}, {"mutex", *mutexProfile}} {
			if p.path == "" {
				continue
			}
			f, err := os.Create(p.path)
			if err == nil {
				err = pprof.Lookup(p.name).WriteTo(f, 0)
				f.Close()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
		}
		if *cpuProfile != "" || *blockProfile != "" || *mutexProfile != "" {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			fmt.Printf("memory: %.1f GB from the system, %d garbage collections taking %.1fs in all\n",
				float64(m.Sys)/(1<<30), m.NumGC, float64(m.PauseTotalNs)/1e9)
		}
	}
}
