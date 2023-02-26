package heapdump

import (
	"fmt"
	"io"
	"strconv"
)

var nameMap map[uint64]string
var oidMap map[uint64]string

func init() {
	nameMap = make(map[uint64]string)
	oidMap = make(map[uint64]string)
}

func AddOid(oid uint64, name string) {
	oidMap[oid] = name
}

func AddName(addr uint64, name string) {
	nameMap[addr] = name
}

func GetName(addr uint64) string {
	name, found := nameMap[addr]
	if found {
		return name
	}
	return ""
}

func ReadOids(r io.Reader) error {
	var oid uint64
	var name string
	for {
		n, err := fmt.Fscanln(r, &oid, &name)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if n == 2 && oid > 0 && len(name) > 0 {
			oidMap[oid] = name
		}
	}
	return nil
}

func ReadSymbols(r io.Reader) error {
	var addr, kind, name string
	for {
		n, err := fmt.Fscanln(r, &addr, &kind, &name)
		if err == io.EOF {
			break
		}
		if err == nil && n == 3 {
			addrInt, err := strconv.ParseUint(addr, 16, 64)
			if err == nil {
				nameMap[addrInt] = name
			}
		}
	}
	return nil
}

// Print out address and, if relevant, the name of what resides there
type Addr uint64

func (a Addr) String() string {
	name, found := nameMap[uint64(a)]
	if found {
		return fmt.Sprintf("0x%x (%s)", uint64(a), name)
	}
	return fmt.Sprintf("0x%x", uint64(a))
}
