package config

import (
	"flag"
	"fmt"
	"os"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type Config struct {
	Dumpfile string
	Output   string
	Oid      string
	Program  string
	Address  uint64
	Children bool
	Print    bool
	Find     string
	Hexdump  bool
	Anchors  bool
	Owners   int
	MakeDump string
}

func Initialize() (*Config, error) {

	flag.String("dumpfile", "", "Heap dump file to read")
	flag.String("output", "heapdump.svg", "Output file")
	flag.String("oid", "", "File that maps from OIDs to object names")
	flag.String("program", "", "File to read symbol information from")
	flag.Int("address", 0, "Address of object to analyze")
	// flag.Bool("children", false, "If set, will show children rather than parents")
	flag.Bool("print", false, "If set, will list all dumpfile records and exit")
	flag.String("find", "", "Finds an object whose name matches the specified regular expression")
	flag.Bool("hexdump", false, "If set, will print a hexdump of the specified object and exit")
	flag.Bool("anchors", false, "If set, will print a list of the anchors keeping the indicated object alive")
	flag.Int("owners", 0, "If positive, will print the owners of the specified object to the depth indicated, and exit; if negative, will print owners to their full depth")
	flag.String("makedump", "", "For debugging and examples: dump heapspurs' heap")

	v := viper.New()
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.CommandLine.MarkHidden("dumpfile")
	pflag.CommandLine.MarkHidden("makedump")
	pflag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s [dumpfile]\n", os.Args[0])
		pflag.PrintDefaults()
	}
	pflag.Parse()
	v.BindPFlags(pflag.CommandLine)

	conf := &Config{}
	err := v.Unmarshal(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	args := pflag.Args()
	if len(args) > 0 {
		conf.Dumpfile = args[0]
	} else if len(conf.Dumpfile) == 0 {
		pflag.Usage()
		os.Exit(-1)
	}
	return conf, nil
}
