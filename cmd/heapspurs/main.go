package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"runtime/debug"

	"github.com/adamroach/heapspurs/internal/pkg/config"
	"github.com/adamroach/heapspurs/pkg/heapdump"
	"github.com/adamroach/heapspurs/pkg/treeclimber"
)

func main() {
	conf, err := config.Initialize()
	if err != nil {
		panic(fmt.Sprintf("Config: %v\n", err))
	}

	if len(conf.Oid) > 0 {
		file, err := os.Open(conf.Oid)
		if err != nil {
			panic(fmt.Sprintf("Open OID file '%s': %v\n", conf.Oid, err))
		}
		err = heapdump.ReadOids(file)
		if err != nil {
			panic(fmt.Sprintf("Reading OID file '%s': %v\n", conf.Oid, err))
		}
		file.Close()
	}

	if len(conf.Program) > 0 {
		cmd := exec.Command("go", "tool", "nm", conf.Program)
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			panic(fmt.Sprintf("Open program file '%s': %v\n", conf.Program, err))
		}
		err = cmd.Start()
		if err != nil {
			panic(fmt.Sprintf("Running [go tool nm] on '%s': %v\n", conf.Program, err))
		}
		if err != nil {
			panic(fmt.Sprintf("Open program file '%s': %v\n", conf.Program, err))
		}
		err = heapdump.ReadSymbols(stdout)
		if err != nil {
			panic(fmt.Sprintf("Reading program file '%s': %v\n", conf.Program, err))
		}
		cmd.Wait()
	}

	file, err := os.Open(conf.Dumpfile)
	if err != nil {
		panic(fmt.Sprintf("Open '%s': %v\n", conf.Dumpfile, err))
	}
	reader := bufio.NewReader(file)

	if conf.Print {
		err = heapdump.PrintRecords(reader, "")
		if err != nil {
			panic(err)
		}
		return
	}

	if len(conf.Find) > 0 {
		err = heapdump.PrintRecords(reader, conf.Find)
		if err != nil {
			panic(err)
		}
		return
	}

	climber, err := treeclimber.NewTreeClimber(reader)

	if len(conf.MakeDump) > 0 {
		f, err := os.Create(conf.MakeDump)
		if err != nil {
			panic("Could not open file for writing:" + err.Error())
		} else {
			runtime.GC()
			debug.WriteHeapDump(f.Fd())
			f.Close()
		}
		return
	}

	if err != nil {
		panic(err)
	}
	file.Close()

	if conf.Anchors {
		err := climber.PrintAnchors(conf.Address)
		if err != nil {
			panic(err)
		}
		return
	}

	if conf.Owners != 0 {
		err := climber.PrintOwners(conf.Address, conf.Owners)
		if err != nil {
			panic(err)
		}
		return
	}

	if conf.Hexdump {
		hexdump, err := climber.Hexdump(conf.Address)
		if err != nil {
			panic(err)
		}
		fmt.Print(hexdump)
		return
	}

	out, err := os.Create(conf.Output)
	if err != nil {
		panic(fmt.Sprintf("Create '%s': %v\n", conf.Output, err))
	}
	climber.WriteSVG(conf.Address, out)
	out.Close()
}
