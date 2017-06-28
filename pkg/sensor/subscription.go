package sensor

import (
	"github.com/capsule8/reactive8/pkg/api/event"
	"github.com/gobwas/glob"
)

func subscriptionMatchEvent(s *event.Subscription, e *event.Event) bool {
	if s.EventFilter == nil {
		return false
	}

	switch e.Event.(type) {
	case *event.Event_Syscall:
		sev := e.GetSyscall()

		for _, sef := range s.EventFilter.SyscallEvents {
			if sef.Type != sev.Type {
				continue
			}

			if sef.Id != nil && sef.Id.Value != sev.Id {
				continue
			}

			if sef.Arg0 != nil && sef.Arg0.Value != sev.Arg0 {
				continue
			}

			if sef.Arg1 != nil && sef.Arg1.Value != sev.Arg1 {
				continue
			}

			if sef.Arg2 != nil && sef.Arg2.Value != sev.Arg2 {
				continue
			}

			if sef.Arg3 != nil && sef.Arg3.Value != sev.Arg3 {
				continue
			}

			if sef.Arg4 != nil && sef.Arg4.Value != sev.Arg4 {
				continue
			}

			if sef.Arg5 != nil && sef.Arg5.Value != sev.Arg5 {
				continue
			}

			if sef.Ret != nil && sef.Ret.Value != sev.Ret {
				continue
			}

			return true
		}

	case *event.Event_Process:
		pev := e.GetProcess()

		for _, pef := range s.EventFilter.ProcessEvents {
			if pef.Type != pev.Type {
				continue
			}

			return true
		}

	case *event.Event_File:
		fev := e.GetFile()

		for _, fef := range s.EventFilter.FileEvents {
			if fef.Type != fev.Type {
				continue
			}

			if fef.Filename != nil {
				if fef.Filename.Value == fev.Filename {
					continue
				}
			}

			if fef.FilenamePattern != nil {
				pattern := fef.FilenamePattern.Value
				g, err := glob.Compile(pattern)
				if err != nil {
					continue
				}

				if !g.Match(fev.Filename) {
					continue
				}
			}

			if fef.OpenFlagsMask != nil {
				m := fef.OpenFlagsMask.Value
				if (m & fev.OpenFlags) == 0 {
					continue
				}
			}

			if fef.CreateModeMask != nil {
				m := fef.CreateModeMask.Value
				if (m & fev.OpenMode) == 0 {
					continue
				}
			}

			return true
		}

	case *event.Event_Container:
		cev := e.GetContainer()

		for _, cef := range s.EventFilter.ContainerEvents {
			if cef.Type != cev.Type {
				continue
			}

			return true
		}
	}

	return false
}
