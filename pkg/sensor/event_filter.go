package sensor

import (
	api "github.com/capsule8/reactive8/pkg/api/v0"
	"github.com/gobwas/glob"
)

type eventFilter struct {
	ef *api.EventFilter
}

func (ef *eventFilter) filterEvent(i interface{}) bool {
	e := i.(*api.Event)

	switch e.Event.(type) {
	case *api.Event_Syscall:
		sev := e.GetSyscall()

		for _, sef := range ef.ef.SyscallEvents {
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

	case *api.Event_Process:
		pev := e.GetProcess()

		for _, pef := range ef.ef.ProcessEvents {
			if pef.Type != pev.Type {
				continue
			}

			return true
		}

	case *api.Event_File:
		fev := e.GetFile()

		for _, fef := range ef.ef.FileEvents {
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

	case *api.Event_Container:
		cev := e.GetContainer()

		for _, cef := range ef.ef.ContainerEvents {
			if cef.Type != cev.Type {
				continue
			}

			return true
		}

	case *api.Event_Chargen:
		// These debugging events are only enabled when the subscription
		// specifies a ChargenEventFilter, so we don't actually need to
		// filter here.

		return true

	case *api.Event_Ticker:
		// These debugging events are only enabled when the subscription
		// specifies a TickerEventFilter, so we don't actually need to
		// filter here.

		return true
	}

	return false
}

func NewEventFilter(ef *api.EventFilter) *eventFilter {
	return &eventFilter{
		ef: ef,
	}
}
