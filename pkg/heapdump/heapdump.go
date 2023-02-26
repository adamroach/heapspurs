package heapdump

// See https://github.com/golang/go/wiki/heapdump15-through-heapdump17

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
)

type Record interface {
	Read(r *bufio.Reader) error
}

type Addressable interface {
	GetAddress() uint64
}

type Owner interface {
	Addressable
	GetContents() []byte
	GetFields() []uint64
}

type RecordType int

const (
	EofType                    RecordType = 0
	ObjectType                 RecordType = 1
	OtherRootType              RecordType = 2
	TypeDescriptorType         RecordType = 3
	GoroutineType              RecordType = 4
	StackFrameType             RecordType = 5
	DumpParamsType             RecordType = 6
	RegisteredFinalizerType    RecordType = 7
	ItabType                   RecordType = 8
	OsThreadType               RecordType = 9
	MemStatsType               RecordType = 10
	QueuedFinalizerType        RecordType = 11
	DataSegmentType            RecordType = 12
	BssSegmentType             RecordType = 13
	DeferRecordType            RecordType = 14
	PanicRecordType            RecordType = 15
	AllocFreeProfileRecordType RecordType = 16
	AllocStackTraceSampleType  RecordType = 17
)

const Header = "go1.7 heap dump\n"

func ReadHeader(reader *bufio.Reader) (err error) {
	val := make([]byte, len(Header))
	n, err := io.ReadFull(reader, val)
	if err != nil {
		return
	}
	if n != len(Header) {
		err = fmt.Errorf("Bad read: expected %d bytes, read %d", len(Header), n)
		return
	}
	if !bytes.Equal(val, []byte(Header)) {
		err = fmt.Errorf("Bad read: expected string '%s', read '%s'", Header, string(val))
		return
	}
	return
}

func ReadRecord(reader *bufio.Reader) (record Record, err error) {
	rt, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	switch RecordType(rt) {
	case EofType:
		record = &Eof{}
	case ObjectType:
		record = &Object{}
	case OtherRootType:
		record = &OtherRoot{}
	case TypeDescriptorType:
		record = &TypeDescriptor{}
	case GoroutineType:
		record = &Goroutine{}
	case StackFrameType:
		record = &StackFrame{}
	case DumpParamsType:
		record = &DumpParams{}
	case RegisteredFinalizerType:
		record = &RegisteredFinalizer{}
	case ItabType:
		record = &Itab{}
	case OsThreadType:
		record = &OsThread{}
	case MemStatsType:
		record = &MemStats{}
	case QueuedFinalizerType:
		record = &QueuedFinalizer{}
	case DataSegmentType:
		record = &DataSegment{}
	case BssSegmentType:
		record = &BssSegment{}
	case DeferRecordType:
		record = &DeferRecord{}
	case PanicRecordType:
		record = &PanicRecord{}
	case AllocFreeProfileRecordType:
		record = &AllocFreeProfileRecord{}
	case AllocStackTraceSampleType:
		record = &AllocStackTraceSample{}
	default:
		return nil, fmt.Errorf("Unexpected record type: %v", rt)
	}

	err = record.Read(reader)

	return
}

func GetPointers(o Owner, p *DumpParams) (pointers []uint64) {
	_, pointers = GetPointerInfo(o, p)
	return
}

func GetPointersSourceAddress(o Owner, target uint64, p *DumpParams) uint64 {
	s, t := GetPointerInfo(o, p)
	for i, t := range t {
		if t == target {
			return s[i]
		}
	}
	return 0
}

func GetPointerInfo(o Owner, p *DumpParams) (pointerSource, pointerTarget []uint64) {
	var byteOrder binary.ByteOrder = binary.LittleEndian
	if p.BigEndian {
		byteOrder = binary.BigEndian
	}
	contents := o.GetContents()
	fields := o.GetFields()
	pointerSource = make([]uint64, len(fields))
	pointerTarget = make([]uint64, len(fields))
	for i := 0; i < len(fields); i++ {
		offset := fields[i]
		pointerSource[i] = o.GetAddress() + offset
		switch p.PointerSize {
		case 2:
			pointerTarget[i] = uint64(byteOrder.Uint16(contents[offset:]))
		case 4:
			pointerTarget[i] = uint64(byteOrder.Uint32(contents[offset:]))
		case 8:
			pointerTarget[i] = byteOrder.Uint64(contents[offset:])
		default:
			panic(fmt.Sprintf("Cannot handle pointers of size %d", p.PointerSize))
		}
	}
	return
}

///////////////////////////////////////////////////////////////////////////

type Eof struct {
}

func (r *Eof) String() string {
	return "End Of File"
}

func (r *Eof) Read(reader *bufio.Reader) (err error) {
	return
}

type Object struct {
	Address  uint64   // address of object
	Contents []byte   // contents of object
	Fields   []uint64 // describes pointer-containing fields of the object
	Name     string
}

func (r *Object) GetAddress() uint64 {
	return r.Address
}

func (r *Object) GetContents() []byte {
	return r.Contents
}

func (r *Object) GetFields() []uint64 {
	return r.Fields
}

func (r *Object) GetName() string {
	if len(r.Name) > 0 {
		return r.Name
	}
	return "Object"
}

func (r *Object) String() string {
	return fmt.Sprintf("%s @ 0x%x with %d pointers in %d bytes", r.GetName(), r.Address, len(r.Fields), len(r.Contents))
}

func (r *Object) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Contents as bytes
	ContentsLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Contents = make([]byte, ContentsLen)
	_, err = io.ReadFull(reader, r.Contents)
	if err != nil {
		return
	}

	// Read Fields as fieldlist
	r.Fields = make([]uint64, 0)
	var kind uint64
	for {
		kind, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		if kind == 0 {
			break
		}
		var value uint64
		value, err = binary.ReadUvarint(reader)
		if kind == 0 {
			break
		}
		r.Fields = append(r.Fields, value)
	}

	// Assign a class name if this object starts with an OID
	if len(r.Contents) > 8 {
		oid := binary.LittleEndian.Uint64(r.Contents[:])
		className, found := oidMap[oid]
		if found {
			r.Name = className
			AddName(r.Address, className)
		}
	}

	return
}

type OtherRoot struct {
	Description string // textual description of where this root came from
	Address     uint64 // root pointer
}

func (r *OtherRoot) String() string {
	return fmt.Sprintf("OtherRoot @ 0x%x: %s", r.Address, r.Description)
}

func (r *OtherRoot) GetAddress() uint64 {
	return r.Address
}

func (r *OtherRoot) Read(reader *bufio.Reader) (err error) {
	// Read Description as string
	DescriptionLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	DescriptionBuf := make([]byte, DescriptionLen)
	_, err = io.ReadFull(reader, DescriptionBuf)
	if err != nil {
		return
	}
	r.Description = string(DescriptionBuf)

	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type TypeDescriptor struct {
	Address  uint64 // address of type descriptor
	TypeSize uint64 // size of an object of this type
	Name     string // name of type
	Indirect bool   // whether the data field of an interface containing a value of this type has type T (false) or *T (true)
}

func (r *TypeDescriptor) GetAddress() uint64 {
	return r.Address
}

func (r *TypeDescriptor) String() string {
	return fmt.Sprintf("TypeDescriptor for '%s' @ 0x%x: Objects are %d bytes", r.Name, r.Address, r.TypeSize)
}

func (r *TypeDescriptor) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read TypeSize as uvarint
	r.TypeSize, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Name as string
	NameLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	NameBuf := make([]byte, NameLen)
	_, err = io.ReadFull(reader, NameBuf)
	if err != nil {
		return
	}
	r.Name = string(NameBuf)

	// Read Indirect as bool
	IndirectInt, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Indirect = (IndirectInt != 0)

	return
}

type Goroutine struct {
	Address                   uint64     // address of descriptor
	StackPointer              uint64     // pointer to the top of stack (the currently running frame, a.k.a. depth 0)
	RoutineId                 uint64     // go routine ID
	CreatorPointer            uint64     // the location of the go statement that created this goroutine
	Status                    StatusType // status
	System                    bool       // is a Go routine started by the system
	Background                bool       // is a background Go routine
	WaitStart                 uint64     // approximate time the go routine last started waiting (nanoseconds since the Epoch)
	WaitReason                string     // textual reason why it is waiting
	CurrentContextPointer     uint64     // context pointer of currently running frame
	OsThreadDescriptorAddress uint64     // address of os thread descriptor
	TopDefer                  uint64     // top defer record
	TopPanic                  uint64     // top panic record
}

func (r *Goroutine) GetAddress() uint64 {
	return r.Address
}

func (r *Goroutine) String() string {
	if r.Status == Waiting {
		return fmt.Sprintf("Goroutine[%d] @ 0x%x: %s (%s), Stack @ 0x%x", r.RoutineId, r.Address, r.Status.String(), r.WaitReason, r.StackPointer)
	}
	return fmt.Sprintf("Goroutine[%d] @ 0x%x: %s, Stack @ 0x%x", r.RoutineId, r.Address, r.Status.String(), r.StackPointer)
}

type StatusType uint64

const (
	Idle     StatusType = 0
	Runnable StatusType = 1
	Syscall  StatusType = 3
	Waiting  StatusType = 4
)

func (s StatusType) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Runnable:
		return "Runnable"
	case Syscall:
		return "Syscall"
	case Waiting:
		return "Waiting"
	}
	return fmt.Sprintf("Unknown status %d", uint64(s))
}

func (r *Goroutine) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read StackPointer as uvarint
	r.StackPointer, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read RoutineId as uvarint
	r.RoutineId, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read CreatorPointer as uvarint
	r.CreatorPointer, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Status as uvarint
	var Status uint64
	Status, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Status = StatusType(Status)

	// Read System as bool
	SystemInt, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.System = (SystemInt != 0)

	// Read Background as bool
	BackgroundInt, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Background = (BackgroundInt != 0)

	// Read WaitStart as uvarint
	r.WaitStart, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read WaitReason as string
	WaitReasonLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	WaitReasonBuf := make([]byte, WaitReasonLen)
	_, err = io.ReadFull(reader, WaitReasonBuf)
	if err != nil {
		return
	}
	r.WaitReason = string(WaitReasonBuf)

	// Read CurrentContextPointer as uvarint
	r.CurrentContextPointer, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read OsThreadDescriptorAddress as uvarint
	r.OsThreadDescriptorAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read TopDefer as uvarint
	r.TopDefer, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read TopPanic as uvarint
	r.TopPanic, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type StackFrame struct {
	Address        uint64   // stack pointer (lowest address in frame)
	Depth          uint64   // depth in stack (0 = top of stack)
	ChildPointer   uint64   // stack pointer of child frame (or 0 if none)
	Contents       []byte   // contents of stack frame
	EntryPc        uint64   // entry pc for function
	CurrentPc      uint64   // current pc for function
	ContinuationPc uint64   // continuation pc for function (where function may resume, if anywhere)
	Name           string   // function name
	Fields         []uint64 // list of kind and offset of pointer-containing fields in this frame
}

func (r *StackFrame) GetAddress() uint64 {
	return r.Address
}

func (r *StackFrame) GetContents() []byte {
	return r.Contents
}

func (r *StackFrame) GetFields() []uint64 {
	return r.Fields
}

func (r *StackFrame) String() string {
	return fmt.Sprintf("StackFrame[%d] @ 0x%x: %s with %d pointers in %d bytes; child = 0x%x",
		r.Depth, r.Address, r.Name, len(r.Fields), len(r.Contents), r.ChildPointer,
	)
}

func (r *StackFrame) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Depth as uvarint
	r.Depth, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read ChildPointer as uvarint
	r.ChildPointer, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Contents as bytes
	ContentsLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Contents = make([]byte, ContentsLen)
	_, err = io.ReadFull(reader, r.Contents)
	if err != nil {
		return
	}

	// Read EntryPc as uvarint
	r.EntryPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read CurrentPc as uvarint
	r.CurrentPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read ContinuationPc as uvarint
	r.ContinuationPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Name as string
	NameLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	NameBuf := make([]byte, NameLen)
	_, err = io.ReadFull(reader, NameBuf)
	if err != nil {
		return
	}
	r.Name = string(NameBuf)

	// Read Fields as fieldlist
	r.Fields = make([]uint64, 0)
	var kind uint64
	for {
		kind, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		if kind == 0 {
			break
		}
		var value uint64
		value, err = binary.ReadUvarint(reader)
		if kind == 0 {
			break
		}
		r.Fields = append(r.Fields, value)
	}

	return
}

type DumpParams struct {
	BigEndian    bool   // big endian
	PointerSize  uint64 // pointer size in bytes
	HeapStart    uint64 // starting address of heap
	HeapEnd      uint64 // ending address of heap
	Architecture string // architecture name
	GoExperiment string // GOEXPERIMENT environment variable value
	Ncpu         uint64 // runtime.ncpu
}

func (r *DumpParams) String() string {
	return fmt.Sprintf("DumpParams: BigEndian=%v, PointerSize=%d, Heap=0x%x-0x%x, Architecture=%s, GOEXPERIMENT=%s, Cpus=%d",
		r.BigEndian,
		r.PointerSize,
		r.HeapStart,
		r.HeapEnd,
		r.Architecture,
		r.GoExperiment,
		r.Ncpu,
	)
}

func (r *DumpParams) Read(reader *bufio.Reader) (err error) {
	// Read BigEndian as bool
	BigEndianInt, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.BigEndian = (BigEndianInt != 0)

	// Read PointerSize as uvarint
	r.PointerSize, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapStart as uvarint
	r.HeapStart, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapEnd as uvarint
	r.HeapEnd, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Architecture as string
	ArchitectureLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	ArchitectureBuf := make([]byte, ArchitectureLen)
	_, err = io.ReadFull(reader, ArchitectureBuf)
	if err != nil {
		return
	}
	r.Architecture = string(ArchitectureBuf)

	// Read GoExperiment as string
	GoExperimentLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	GoExperimentBuf := make([]byte, GoExperimentLen)
	_, err = io.ReadFull(reader, GoExperimentBuf)
	if err != nil {
		return
	}
	r.GoExperiment = string(GoExperimentBuf)

	// Read Ncpu as uvarint
	r.Ncpu, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type RegisteredFinalizer struct {
	ObjectAddress    uint64 // address of object that has a finalizer
	FinalizerAddress uint64 // pointer to FuncVal describing the finalizer
	FinalizerEntryPc uint64 // PC of finalizer entry point
	FinalizerType    uint64 // type of finalizer argument
	ObjectType       uint64 // type of object
}

func (r *RegisteredFinalizer) String() string {
	return fmt.Sprintf("RegisteredFinalizer @ 0x%x: FuncVal: 0x%x, Type: 0x%x, Object Type: 0x%x",
		r.ObjectAddress,
		r.FinalizerAddress,
		r.FinalizerType,
		r.ObjectType,
	)
}

func (r *RegisteredFinalizer) Read(reader *bufio.Reader) (err error) {
	// Read ObjectAddress as uvarint
	r.ObjectAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerAddress as uvarint
	r.FinalizerAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerEntryPc as uvarint
	r.FinalizerEntryPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerType as uvarint
	r.FinalizerType, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read ObjectType as uvarint
	r.ObjectType, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type Itab struct {
	Address               uint64 // Itab address
	TypeDescriptorAddress uint64 // address of type descriptor for contained type
}

func (r *Itab) GetAddress() uint64 {
	return r.Address
}

func (r *Itab) String() string {
	return fmt.Sprintf("Itab @ 0x%x: 0x%x", r.Address, r.TypeDescriptorAddress)
}

func (r *Itab) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read TypeDescriptorAddress as uvarint
	r.TypeDescriptorAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type OsThread struct {
	ThreadDescriptorAddress uint64 // address of this os thread descriptor
	GoId                    uint64 // Go internal id of thread
	OsId                    uint64 // os's id for thread
}

func (r *OsThread) String() string {
	return fmt.Sprintf("OsThread @ 0x%x: GoId = %d; OsId = 0x%x", r.ThreadDescriptorAddress, r.GoId, r.OsId)
}

func (r *OsThread) Read(reader *bufio.Reader) (err error) {
	// Read ThreadDescriptorAddress as uvarint
	r.ThreadDescriptorAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read GoId as uvarint
	r.GoId, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read OsId as uvarint
	r.OsId, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type MemStats struct {
	Alloc        uint64
	TotalAlloc   uint64
	Sys          uint64
	Lookups      uint64
	Mallocs      uint64
	Frees        uint64
	HeapAlloc    uint64
	HeapSys      uint64
	HeapIdle     uint64
	HeapInuse    uint64
	HeapReleased uint64
	HeapObjects  uint64
	StackInuse   uint64
	StackSys     uint64
	MSpanInuse   uint64
	MSpanSys     uint64
	MCacheInuse  uint64
	MCacheSys    uint64
	BuckHashSys  uint64
	GCSys        uint64
	OtherSys     uint64
	NextGC       uint64
	LastGC       uint64
	PauseTotalNs uint64
	PauseNs      [256]uint64
	NumGC        uint64
}

func (r *MemStats) String() string {
	return fmt.Sprintf("MemStats: %+v", *r)
}

func (r *MemStats) Read(reader *bufio.Reader) (err error) {
	// Read Alloc as uvarint
	r.Alloc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read TotalAlloc as uvarint
	r.TotalAlloc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Sys as uvarint
	r.Sys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Lookups as uvarint
	r.Lookups, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Mallocs as uvarint
	r.Mallocs, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Frees as uvarint
	r.Frees, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapAlloc as uvarint
	r.HeapAlloc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapSys as uvarint
	r.HeapSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapIdle as uvarint
	r.HeapIdle, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapInuse as uvarint
	r.HeapInuse, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapReleased as uvarint
	r.HeapReleased, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read HeapObjects as uvarint
	r.HeapObjects, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read StackInuse as uvarint
	r.StackInuse, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read StackSys as uvarint
	r.StackSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read MSpanInuse as uvarint
	r.MSpanInuse, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read MSpanSys as uvarint
	r.MSpanSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read MCacheInuse as uvarint
	r.MCacheInuse, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read MCacheSys as uvarint
	r.MCacheSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read BuckHashSys as uvarint
	r.BuckHashSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read GCSys as uvarint
	r.GCSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read OtherSys as uvarint
	r.OtherSys, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read NextGC as uvarint
	r.NextGC, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read LastGC as uvarint
	r.LastGC, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read PauseTotalNs as uvarint
	r.PauseTotalNs, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read PauseNs as 256uvarints
	for i := 0; i < 256; i++ {
		r.PauseNs[i], err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
	}

	// Read NumGC as uvarint
	r.NumGC, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type QueuedFinalizer struct {
	ObjectAddress    uint64 // address of object that has a finalizer
	FinalizerAddress uint64 // pointer to FuncVal describing the finalizer
	FinalizerEntryPc uint64 // PC of finalizer entry point
	FinalizerType    uint64 // type of finalizer argument
	ObjectType       uint64 // type of object
}

func (r *QueuedFinalizer) String() string {
	return fmt.Sprintf("QueuedFinalizer @ 0x%x: FuncVal: 0x%x, Type: 0x%x, Object Type: 0x%x",
		r.ObjectAddress,
		r.FinalizerAddress,
		r.FinalizerType,
		r.ObjectType,
	)
}

func (r *QueuedFinalizer) Read(reader *bufio.Reader) (err error) {
	// Read ObjectAddress as uvarint
	r.ObjectAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerAddress as uvarint
	r.FinalizerAddress, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerEntryPc as uvarint
	r.FinalizerEntryPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FinalizerType as uvarint
	r.FinalizerType, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read ObjectType as uvarint
	r.ObjectType, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type DataSegment struct {
	Address  uint64   // address of the start of the data segment
	Contents []byte   // contents of the data segment
	Fields   []uint64 // kind and offset of pointer-containing fields in the data segment.
}

func (r *DataSegment) GetAddress() uint64 {
	return r.Address
}

func (r *DataSegment) GetContents() []byte {
	return r.Contents
}

func (r *DataSegment) GetFields() []uint64 {
	return r.Fields
}

func (r *DataSegment) String() string {
	return fmt.Sprintf("DataSegment @ 0x%x-0x%x with %d pointers", r.Address, r.Address+uint64(len(r.Contents)), len(r.Fields))
}

func (r *DataSegment) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Contents as bytes
	ContentsLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Contents = make([]byte, ContentsLen)
	_, err = io.ReadFull(reader, r.Contents)
	if err != nil {
		return
	}

	// Read Fields as fieldlist
	r.Fields = make([]uint64, 0)
	var kind uint64
	for {
		kind, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		if kind == 0 {
			break
		}
		var value uint64
		value, err = binary.ReadUvarint(reader)
		if kind == 0 {
			break
		}
		r.Fields = append(r.Fields, value)
	}

	return
}

type BssSegment struct {
	Address  uint64   // address of the start of the data segment
	Contents []byte   // contents of the data segment
	Fields   []uint64 // kind and offset of pointer-containing fields in the data segment.
}

func (r *BssSegment) GetAddress() uint64 {
	return r.Address
}

func (r *BssSegment) GetContents() []byte {
	return r.Contents
}

func (r *BssSegment) GetFields() []uint64 {
	return r.Fields
}

func (r *BssSegment) String() string {
	return fmt.Sprintf("BssSegment @ 0x%x-0x%x with %d pointers", r.Address, r.Address+uint64(len(r.Contents)), len(r.Fields))
}

func (r *BssSegment) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Contents as bytes
	ContentsLen, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Contents = make([]byte, ContentsLen)
	_, err = io.ReadFull(reader, r.Contents)
	if err != nil {
		return
	}

	// Read Fields as fieldlist
	r.Fields = make([]uint64, 0)
	var kind uint64
	for {
		kind, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		if kind == 0 {
			break
		}
		var value uint64
		value, err = binary.ReadUvarint(reader)
		if kind == 0 {
			break
		}
		r.Fields = append(r.Fields, value)
	}

	return
}

type DeferRecord struct {
	Address             uint64 // defer record address
	ContainingGoroutine uint64 // containing goroutine
	Arcp                uint64 // argp
	Pc                  uint64 // pc
	FuncVal             uint64 // FuncVal of defer
	EntryPointPc        uint64 // PC of defer entry point
	Next                uint64 // link to next defer record
}

func (r *DeferRecord) GetAddress() uint64 {
	return r.Address
}

func (r *DeferRecord) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read ContainingGoroutine as uvarint
	r.ContainingGoroutine, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Arcp as uvarint
	r.Arcp, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Pc as uvarint
	r.Pc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FuncVal as uvarint
	r.FuncVal, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read EntryPointPc as uvarint
	r.EntryPointPc, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Next as uvarint
	r.Next, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type PanicRecord struct {
	Address        uint64 // panic record address
	Goroutine      uint64 // containing goroutine
	PanicArgType   uint64 // type ptr of panic arg eface
	PanicArgData   uint64 // data field of panic arg eface
	DeferRecordPtr uint64 // ptr to defer record that's currently running
	Next           uint64 // link to next panic record
}

func (r *PanicRecord) GetAddress() uint64 {
	return r.Address
}

func (r *PanicRecord) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Goroutine as uvarint
	r.Goroutine, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read PanicArgType as uvarint
	r.PanicArgType, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read PanicArgData as uvarint
	r.PanicArgData, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read DeferRecordPtr as uvarint
	r.DeferRecordPtr, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Next as uvarint
	r.Next, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type AllocFreeProfileRecord struct {
	Id              uint64  // record identifier
	Size            uint64  // size of allocated object
	Frames          []frame // stack frames
	AllocationCount uint64  // number of allocations
	FreeCount       uint64  // number of frees
}

type frame struct {
	Name     string // function name
	Filename string // file name
	Line     uint64 // line number
}

func (r *AllocFreeProfileRecord) String() string {
	return fmt.Sprintf("AllocFreeProfileRecord: %+v", *r)
}

func (r *AllocFreeProfileRecord) Read(reader *bufio.Reader) (err error) {
	// Read Id as uvarint
	r.Id, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read Size as uvarint
	r.Size, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FrameCount as uvarint
	FrameCount, err := binary.ReadUvarint(reader)
	if err != nil {
		return
	}
	r.Frames = make([]frame, FrameCount)

	for i := uint64(0); i < FrameCount; i++ {
		var NameLen, FilenameLen uint64

		// Read Name as string
		NameLen, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		NameBuf := make([]byte, NameLen)
		_, err = io.ReadFull(reader, NameBuf)
		if err != nil {
			return
		}
		r.Frames[i].Name = string(NameBuf)

		// Read Filename as string
		FilenameLen, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
		FilenameBuf := make([]byte, FilenameLen)
		_, err = io.ReadFull(reader, FilenameBuf)
		if err != nil {
			return
		}
		r.Frames[i].Filename = string(FilenameBuf)

		// Read Line as uvarint
		r.Frames[i].Line, err = binary.ReadUvarint(reader)
		if err != nil {
			return
		}
	}

	// Read AllocationCount as uvarint
	r.AllocationCount, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read FreeCount as uvarint
	r.FreeCount, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}

type AllocStackTraceSample struct {
	Address                  uint64 // address of object
	AllocFreeProfileRecordId uint64 // alloc/free profile record identifier
}

func (r *AllocStackTraceSample) GetAddress() uint64 {
	return r.Address
}

func (r *AllocStackTraceSample) String() string {
	return fmt.Sprintf("AllocStackTraceSample: %+v", *r)
}

func (r *AllocStackTraceSample) Read(reader *bufio.Reader) (err error) {
	// Read Address as uvarint
	r.Address, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	// Read AllocFreeProfileRecordId as uvarint
	r.AllocFreeProfileRecordId, err = binary.ReadUvarint(reader)
	if err != nil {
		return
	}

	return
}
