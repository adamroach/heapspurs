package heapdump

import (
	"bufio"
	"fmt"
	"regexp"
)

func PrintRecords(reader *bufio.Reader, search string) error {

	re, err := regexp.Compile(search)
	if err != nil {
		return fmt.Errorf("Bad regex '%s': %w\n", search, err)
	}

	err = ReadHeader(reader)
	if err != nil {
		return fmt.Errorf("Reading header: %w\n", err)
	}

	var params *DumpParams

	for {
		record, err := ReadRecord(reader)
		if err != nil {
			return (err)
		}
		p, isParams := record.(*DumpParams)
		if isParams {
			params = p
		}

		_, isEof := record.(*Eof)
		obj, isObject := record.(*Object)
		if len(search) > 0 && !isEof && (!isObject || !re.MatchString(obj.Name)) {
			continue
		}
		s, canString := record.(fmt.Stringer)
		if canString {
			fmt.Printf("%s\n", s.String())
		} else {
			fmt.Printf("%T\n", record)
		}
		o, isOwner := record.(Owner)
		if isOwner {
			pointers := GetPointers(o, params)
			for i := 0; i < len(pointers); i++ {
				if pointers[i] != 0 {
					a, _ := record.(Addressable)
					address := a.GetAddress() + o.GetFields()[i]
					fmt.Printf("  Pointer[%d]@%s = %s\n", i, Addr(address), Addr(pointers[i]))
				}
			}
		}
		if isEof {
			break
		}
	}
	return nil
}
