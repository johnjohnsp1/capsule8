package filter

import api "github.com/capsule8/api/v0"

func NewEventFilter(ef *api.EventFilter) Filter {
	return &eventFilter{
		ef: ef,
	}
}

type eventFilter struct {
	ef *api.EventFilter
}

func (ef *eventFilter) FilterFunc(i interface{}) bool {
	e := i.(*api.Event)

	switch e.Event.(type) {
	case *api.Event_Syscall:
		sev := e.GetSyscall()

		for _, sef := range ef.ef.SyscallEvents {
			mappings := make(map[string]*MappedField)
			match, _ := CompareFields(sef, sev, mappings)
			if !match {
				continue
			}
			return true
		}

	case *api.Event_Process:
		pev := e.GetProcess()

		for _, pef := range ef.ef.ProcessEvents {
			mappings := make(map[string]*MappedField)
			match, _ := CompareFields(pef, pev, mappings)
			if !match {
				continue
			}

			return true
		}

	case *api.Event_File:
		fev := e.GetFile()

		for _, fef := range ef.ef.FileEvents {
			mappings := map[string]*MappedField{
				"FilenamePattern": &MappedField{
					Name: "Filename",
					Op:   COMPARISON_PATTERNMATCH,
				},
				"OpenFlagsMask": &MappedField{
					Name: "OpenFlags",
					Op:   COMPARISON_BITAND,
				},
				"CreateModeMask": &MappedField{
					Name: "OpenMode",
					Op:   COMPARISON_BITAND,
				},
			}
			match, _ := CompareFields(fef, fev, mappings)
			if !match {
				continue
			}

			return true
		}

	case *api.Event_KernelCall:
		// The filtering is all handled in the kernel, let it through
		return true

	case *api.Event_Network:
		// The filtering is all handled in the kernel, let it through
		return true

	case *api.Event_Container:
		cev := e.GetContainer()

		for _, cef := range ef.ef.ContainerEvents {
			mappings := make(map[string]*MappedField)
			match, _ := CompareFields(cef, cev, mappings)
			if !match {
				continue
			}

			if cef.View != api.ContainerEventView_FULL {
				cev.OciConfigJson = ""
				cev.DockerConfigJson = ""
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

func (ef *eventFilter) DoFunc(i interface{}) {}
