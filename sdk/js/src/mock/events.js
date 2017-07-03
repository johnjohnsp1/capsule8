// Mock events

const uuid = require("uuid/v4");

const syscall = {
  Syscall: {
    Type: 0,
    Id: 123,
    Arg0: 1,
    Arg1: 2,
    Arg2: 3,
    Arg3: null,
    Arg4: null,
    Arg5: null,
    Ret: 321,
  },
};

const process = {
  Process: {
    Type: 0,
    Pid: 123,
    ChildPid: 456,
    ExecFilename: "maybeexec",
    ExitCode: 0,
  },
};

const file = {
  File: {
    Type: 0,
    Filename: "trustme",
    OpenFlags: 3,
    OpenMode: 666,
  },
};

const container = {
  Container: {
    Type: 0,
    Name: "container event",
    ImageId: 9000,
    ImageName: "toteslegit",
    Pid: 123,
    ExitCode: 0,
  },
};

const Event = (subevent) => {
  return {
    Id: uuid(),
    ContainerId: "container-" + uuid(),
    NodeId: "node-" + uuid(),
    Event: subevent,
  };
}

export const EVENTS = [
  Event(syscall),
  Event(process),
  Event(file),
  Event(container),
];
