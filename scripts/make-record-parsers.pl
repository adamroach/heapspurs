#!/usr/bin/perl

%t = (
  'uvarint' => 'uint64',
  '256uvarints' => '[256]uint64',
  'string' => 'string',
  'bytes' => '[]byte',
  'bool' => 'bool',
  'fieldlist' => '[]uint64',
);

while (<DATA>) {
  chop;
  next if /^$/;
  if (/^(uvarint|string|bytes|bool|256uvarints|fieldlist):([^ ]*) ?(.*)/) {
    $type = $1;
    $name = $2;
    $comment = $3;
    if ($name eq '') {
      $name = "_";
    }
    if ($comment !~ /^ *$/) {
      $fields .= "\t$name $t{$type} // $comment\n";
    } else {
      $fields .= "\t$name $t{$type}\n";
    }

    $loads .= "\t// Read $name as $type\n";
    if ($type eq 'uvarint') {
      $loads .= "\tr.$name, err = binary.ReadUvarint(reader)\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
    }
    elsif ($type eq 'bool') {
      $loads .= "\t${name}Int, err = binary.ReadUvarint(reader)\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
      $loads .= "\tr.$name = (${name}Int != 0)\n";
    }
    elsif ($type eq '256uvarints') {
      $loads .= "\tfor i = 0; i < 256; i ++ {\n";
      $loads .= "\t\tr.${name}\[i], err = binary.ReadUvarint(reader)\n";
      $loads .= "\t\tif err != nil {\n\t\t\treturn\n\t\t}\n";
      $loads .= "\t}\n";
    }
    elsif ($type eq 'string') {
      $loads .= "\t${name}Len, err := binary.ReadUvarint(reader)\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
      $loads .= "\t${name}Buf := make([]byte, ${name}Len)\n";
      $loads .= "\t_, err = io.ReadFull(reader, ${name}Buf)\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
      $loads .= "\tr.$name = string(${name}Buf)\n";
    }
    elsif ($type eq 'bytes') {
      $loads .= "\t${name}Len, err := binary.ReadUvarint(reader)\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
      $loads .= "\tr.${name} = make([]byte, ${name}Len)\n";
      $loads .= "\t_, err = io.ReadFull(reader, r.${name})\n";
      $loads .= "\tif err != nil {\n\t\treturn\n\t}\n";
    }
    elsif ($type eq 'fieldlist') {
      $loads .= "\tr.${name} = make([]uint64, 0)\n";
      $loads .= "\tfor {\n";
      $loads .= "\t\tkind, err := binary.ReadUvarint(reader)\n";
      $loads .= "\t\tif err != nil {\n\t\t\treturn\n\t\t}\n";
      $loads .= "\t\tif kind == 0 {\n\t\t\tbreak\n\t\t}\n";
      $loads .= "\t\tvalue, err := binary.ReadUvarint(reader)\n";
      $loads .= "\t\tif kind == 0 {\n\t\t\tbreak\n\t\t}\n";
      $loads .= "\t\tr.${name} = append(r.${name}, value)\n";
      $loads .= "\t}\n";
    }
    else {
      die $type;
    }
    $loads .= "\n";

  } else {
    if ($class ne '') {
      printclass($class);
    }
    $class = camel($_);
  }
}

printclass($class);

sub printclass {
  $class = shift;
  print "type $class struct {\n";
  print "$fields";
  print "}\n\n";

  print "func (r *$class) Read(reader io.Reader) (err error) {\n";
  print $loads;
  print "\treturn\n}\n\n";

  $fields = "";
  $loads = "";
}

sub camel {
  $s = shift;
  $s =~ y/A-Z/a-z/;
  $s =~ s/ (.)/uc($1)/eg;
  $s =~ s/^(.)/uc($1)/eg;
  return $s;
}

__DATA__
eof

object
uvarint:Address address of object
bytes:Contents contents of object
fieldlist:Fields describes pointer-containing fields of the object

other root
string:Description textual description of where this root came from
uvarint:Address root pointer

type descriptor
uvarint:Address address of type descriptor
uvarint:TypeSize size of an object of this type
string:Name name of type
bool:Indirect whether the data field of an interface containing a value of this type has type T (false) or *T (true)

goroutine
uvarint:Address address of descriptor
uvarint:StackPointer pointer to the top of stack (the currently running frame, a.k.a. depth 0)
uvarint:RoutineId go routine ID
uvarint:CreatorPointer the location of the go statement that created this goroutine
uvarint:Status status
bool:System is a Go routine started by the system
bool:Background is a background Go routine
uvarint:WaitStart approximate time the go routine last started waiting (nanoseconds since the Epoch)
string:WaitReason textual reason why it is waiting
uvarint:CurrentContextPointer context pointer of currently running frame
uvarint:OsThreadDescriptorAddress address of os thread descriptor 
uvarint:TopDefer top defer record
uvarint:TopPanic top panic record

stack frame
uvarint:Address stack pointer (lowest address in frame)
uvarint:Depth depth in stack (0 = top of stack)
uvarint:ChildPointer stack pointer of child frame (or 0 if none)
bytes:Contents contents of stack frame
uvarint:EntryPc entry pc for function
uvarint:CurrentPc current pc for function
uvarint:ContinuationPc continuation pc for function (where function may resume, if anywhere)
string:Name function name
fieldlist:Fields list of kind and offset of pointer-containing fields in this frame

dump params
bool:BigEndian big endian
uvarint:PointerSize pointer size in bytes
uvarint:HeapStart starting address of heap
uvarint:HeapEnd ending address of heap
string:Architecture architecture name
string:GoExperiment GOEXPERIMENT environment variable value
uvarint:Ncpu runtime.ncpu

registered finalizer
uvarint:ObjectAddress address of object that has a finalizer
uvarint:FinalizerAddress pointer to FuncVal describing the finalizer
uvarint:FinalizerEntryPc PC of finalizer entry point
uvarint:FinalizerType type of finalizer argument
uvarint:ObjectType type of object

itab
uvarint:Address Itab address
uvarint:TypeDescriptorAddress address of type descriptor for contained type

osthread
uvarint:ThreadDescriptorAddress address of this os thread descriptor
uvarint:GoId Go internal id of thread
uvarint:OsId os's id for thread

mem stats
uvarint:Alloc
uvarint:TotalAlloc
uvarint:Sys
uvarint:Lookups
uvarint:Mallocs
uvarint:Frees
uvarint:HeapAlloc
uvarint:HeapSys
uvarint:HeapIdle
uvarint:HeapInuse
uvarint:HeapReleased
uvarint:HeapObjects
uvarint:StackInuse
uvarint:StackSys
uvarint:MSpanInuse
uvarint:MSpanSys
uvarint:MCacheInuse
uvarint:MCacheSys
uvarint:BuckHashSys
uvarint:GCSys
uvarint:OtherSys
uvarint:NextGC
uvarint:LastGC
uvarint:PauseTotalNs
256uvarints:PauseNs
uvarint:NumGC

queued finalizer
uvarint:ObjectAddress address of object that has a finalizer
uvarint:FinalizerAddress pointer to FuncVal describing the finalizer
uvarint:FinalizerEntryPc PC of finalizer entry point
uvarint:FinalizerType type of finalizer argument
uvarint:ObjectType type of object

data segment
uvarint:Address address of the start of the data segment
bytes:Contents contents of the data segment
fieldlist:Fields kind and offset of pointer-containing fields in the data segment.

bss segment
uvarint:Address address of the start of the data segment
bytes:Contents contents of the data segment
fieldlist:Fields kind and offset of pointer-containing fields in the data segment.

defer record
uvarint:Address defer record address
uvarint:ContainingGoroutine containing goroutine
uvarint:Arcp argp
uvarint:Pc pc
uvarint:FuncVal FuncVal of defer
uvarint:EntryPointPc PC of defer entry point
uvarint:Next link to next defer record

panic record
uvarint:Address panic record address
uvarint:Goroutine containing goroutine
uvarint:PanicArgType type ptr of panic arg eface
uvarint:PanicArgData data field of panic arg eface
uvarint:DeferRecordPtr ptr to defer record that's currently running
uvarint:Next link to next panic record

alloc free profile record
uvarint:Id record identifier
uvarint:Size size of allocated object
uvarint:FrameCount number of stack frames. For each frame:
string:Name function name
string:Filename file name
uvarint:Line line number
uvarint:AllocationCount number of allocations
uvarint:FreeCount number of frees

alloc stack trace sample
uvarint:Address address of object
uvarint:AllocFreeProfileRecordId alloc/free profile record identifier
