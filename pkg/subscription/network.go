package subscription

import (
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"
	"github.com/capsule8/capsule8/pkg/sys/perf"
	"github.com/golang/glog"
)

const (
	KPROBE_BIND_SYMBOL    = "sys_bind"
	KPROBE_BIND_FETCHARGS = "fd=%di sa_family=+0(%si):u16 sin_port=+2(%si):u16 sin_addr=+4(%si):u32 sun_path=+2(%si):string sin6_port=+2(%si):u16 sin6_addr_high=+8(%si):u64 sin6_addr_low=+16(%si):u64"

	KPROBE_CONNECT_SYMBOL    = "sys_connect"
	KPROBE_CONNECT_FETCHARGS = "fd=%di sa_family=+0(%si):u16 sin_port=+2(%si):u16 sin_addr=+4(%si):u32 sun_path=+2(%si):string sin6_port=+2(%si):u16 sin6_addr_high=+8(%si):u64 sin6_addr_low=+16(%si):u64"

	KPROBE_SENDMSG_SYMBOL    = "sys_sendmsg"
	KPROBE_SENDMSG_FETCHARGS = "fd=%di sa_family=+0(+0(%si)):u16 sin_port=+2(+0(%si)):u16 sin_addr=+4(+0(%si)):u32 sun_path=+2(+0(%si)):string sin6_port=+2(+0(%si)):u16 sin6_addr_high=+8(+0(%si)):u64 sin6_addr_low=+16(+0(%si)):u64"

	KPROBE_SENDTO_SYMBOL    = "sys_sendto"
	KPROBE_SENDTO_FETCHARGS = "fd=%di sa_family=+0(%r8):u16 sin_port=+2(%r8):u16 sin_addr=+4(%r8):u32 sun_path=+2(%r8):string sin6_port=+2(%r8):u16 sin6_addr_high=+8(%r8):u64 sin6_addr_low=+16(%r8):u64"
)

func newNetworkEvent(eventType api.NetworkEventType, sample *perf.SampleRecord, data perf.TraceEventSampleData) *api.Event {
	// If this even contains a network address, throw away any family that
	// we do not support without doing the extra work of creating an event
	// just to throw it away
	family, have_family := data["sa_family"]
	if have_family {
		switch family {
		case 1: // AF_LOCAL
			break
		case 2: // AF_INET
			break
		case 10: // AF_INET6
			break
		default:
			return nil
		}
	}

	event := newEventFromSample(sample, data)
	event.Event = &api.Event_Network{
		Network: &api.NetworkEvent{
			Type: eventType,
		},
	}
	network := event.Event.(*api.Event_Network).Network

	if have_family {
		switch family {
		case 1: // AF_LOCAL
			network.Address = &api.NetworkAddress{
				Family: api.NetworkAddressFamily_NETWORK_ADDRESS_FAMILY_LOCAL,
				Address: &api.NetworkAddress_LocalAddress{
					LocalAddress: data["sun_path"].(string),
				},
			}
		case 2: // AF_INET
			network.Address = &api.NetworkAddress{
				Family: api.NetworkAddressFamily_NETWORK_ADDRESS_FAMILY_INET,
				Address: &api.NetworkAddress_Ipv4Address{
					Ipv4Address: &api.IPv4AddressAndPort{
						Address: &api.IPv4Address{
							Address: data["sin_addr"].(uint32),
						},
						Port: uint32(data["sin_port"].(uint16)),
					},
				},
			}
		case 10: // AF_INET6
			network.Address = &api.NetworkAddress{
				Family: api.NetworkAddressFamily_NETWORK_ADDRESS_FAMILY_INET6,
				Address: &api.NetworkAddress_Ipv6Address{
					Ipv6Address: &api.IPv6AddressAndPort{
						Address: &api.IPv6Address{
							High: data["sin6_addr_high"].(uint64),
							Low:  data["sin6_addr_low"].(uint64),
						},
						Port: uint32(data["sin6_port"].(uint16)),
					},
				},
			}
		default:
			// This shouldn't be reachable
			return nil
		}
	}

	fd, ok := data["fd"]
	if ok {
		network.Sockfd = fd.(uint64)
	}

	ret, ok := data["ret"]
	if ok {
		network.Result = ret.(int64)
	}

	return event
}

func decodeSysEnterAccept(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT, sample, data)
	return event, nil
}

func decodeSysExitAccept(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT, sample, data)
	return event, nil
}

func decodeSysBind(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT, sample, data)
	return event, nil
}

func decodeSysExitBind(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT, sample, data)
	return event, nil
}

func decodeSysConnect(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT, sample, data)
	return event, nil
}

func decodeSysExitConnect(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT, sample, data)
	return event, nil
}

func decodeSysEnterListen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT, sample, data)

	network := event.Event.(*api.Event_Network).Network
	network.Backlog = data["backlog"].(uint64)

	return event, nil
}

func decodeSysExitListen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT, sample, data)
	return event, nil
}

func decodeSysEnterRecvfrom(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT, sample, data)
	return event, nil
}

func decodeSysExitRecvfrom(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT, sample, data)
	return event, nil
}

func decodeSysSendto(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT, sample, data)
	return event, nil
}

func decodeSysExitSendto(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT, sample, data)
	return event, nil
}

type networkFilterSet struct {
	acceptAttemptFilters   map[string]int
	acceptResultFilters    map[string]int
	bindAttemptFilters     map[string]int
	bindResultFilters      map[string]int
	connectAttemptFilters  map[string]int
	connectResultFilters   map[string]int
	listenAttemptFilters   map[string]int
	listenResultFilters    map[string]int
	sendtoAttemptFilters   map[string]int
	sendtoResultFilters    map[string]int
	recvfromAttemptFilters map[string]int
	recvfromResultFilters  map[string]int
}

func (nfs *networkFilterSet) add(nef *api.NetworkEventFilter) {
	var filter string

	if nef.Filter != nil {
		filter = filterString(nef.Filter)
		if filter == "" {
			// An empty filter string here is indicative of an error
			// in converting the filter into a valid kernel filter
			// string
			return
		}
	}

	switch nef.Type {
	case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT:
		if nfs.acceptAttemptFilters == nil {
			nfs.acceptAttemptFilters = make(map[string]int)
		}
		nfs.acceptAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT:
		if nfs.acceptResultFilters == nil {
			nfs.acceptResultFilters = make(map[string]int)
		}
		nfs.acceptResultFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT:
		if nfs.bindAttemptFilters == nil {
			nfs.bindAttemptFilters = make(map[string]int)
		}
		nfs.bindAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT:
		if nfs.bindResultFilters == nil {
			nfs.bindResultFilters = make(map[string]int)
		}
		nfs.bindResultFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT:
		if nfs.connectAttemptFilters == nil {
			nfs.connectAttemptFilters = make(map[string]int)
		}
		nfs.connectAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT:
		if nfs.connectResultFilters == nil {
			nfs.connectResultFilters = make(map[string]int)
		}
		nfs.connectResultFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT:
		if nfs.listenAttemptFilters == nil {
			nfs.listenAttemptFilters = make(map[string]int)
		}
		nfs.listenAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT:
		if nfs.listenResultFilters == nil {
			nfs.listenResultFilters = make(map[string]int)
		}
		nfs.listenResultFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT:
		if nfs.recvfromAttemptFilters == nil {
			nfs.recvfromAttemptFilters = make(map[string]int)
		}
		nfs.recvfromAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT:
		if nfs.recvfromResultFilters == nil {
			nfs.recvfromResultFilters = make(map[string]int)
		}
		nfs.recvfromResultFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT:
		if nfs.sendtoAttemptFilters == nil {
			nfs.sendtoAttemptFilters = make(map[string]int)
		}
		nfs.sendtoAttemptFilters[filter]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT:
		if nfs.sendtoResultFilters == nil {
			nfs.sendtoResultFilters = make(map[string]int)
		}
		nfs.sendtoResultFilters[filter]++
	}
}

func (nfs *networkFilterSet) len() int {
	length :=
		len(nfs.acceptAttemptFilters) +
			len(nfs.acceptResultFilters) +
			len(nfs.bindAttemptFilters) +
			len(nfs.bindResultFilters) +
			len(nfs.connectAttemptFilters) +
			len(nfs.connectResultFilters) +
			len(nfs.listenAttemptFilters) +
			len(nfs.sendtoAttemptFilters) +
			len(nfs.sendtoResultFilters) +
			len(nfs.recvfromAttemptFilters) +
			len(nfs.recvfromResultFilters)

	return length
}

func fullFilterString(filters map[string]int) (string, bool) {
	if filters == nil {
		return "", false
	}
	parts := make([]string, 0, len(filters))
	for filter, count := range filters {
		if count <= 0 {
			continue
		}
		if filter == "" {
			return "", true
		}
		parts = append(parts, fmt.Sprintf("(%s)", filter))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " || "), true
	}
	return "", false
}

func registerEvent(monitor *perf.EventMonitor, name string, fn perf.TraceEventDecoderFn, filters map[string]int) {
	filter, active := fullFilterString(filters)
	if !active {
		return
	}

	err := monitor.RegisterEvent(name, fn, filter, nil)
	if err != nil {
		glog.Warningf("Could not register tracepoint %s", name)
	}
}

func registerKprobe(monitor *perf.EventMonitor, symbol string, fetchargs string, fn perf.TraceEventDecoderFn, filters map[string]int) {
	filter, active := fullFilterString(filters)
	if !active {
		return
	}

	name := perf.UniqueProbeName("capsule8", symbol)
	_, err := monitor.RegisterKprobe(name, symbol, false, fetchargs, fn, filter, nil)
	if err != nil {
		glog.Warningf("Could not register network kprobe %s", symbol)
	}
}

func (nfs *networkFilterSet) registerEvents(monitor *perf.EventMonitor) {
	registerEvent(monitor, "syscalls/sys_enter_accept", decodeSysEnterAccept, nfs.acceptAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_accept", decodeSysExitAccept, nfs.acceptResultFilters)

	registerKprobe(monitor, KPROBE_BIND_SYMBOL, KPROBE_BIND_FETCHARGS, decodeSysBind, nfs.bindAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_bind", decodeSysExitBind, nfs.bindResultFilters)

	registerKprobe(monitor, KPROBE_CONNECT_SYMBOL, KPROBE_CONNECT_FETCHARGS, decodeSysConnect, nfs.connectAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_connect", decodeSysExitConnect, nfs.connectResultFilters)

	registerEvent(monitor, "syscalls/sys_enter_listen", decodeSysEnterListen, nfs.listenAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_listen", decodeSysExitListen, nfs.listenResultFilters)

	// There are two additional system calls added in Linux 3.0 that are of
	// interest, but there's no way to get all of the data without eBPF
	// support, so don't bother with them for now.

	registerEvent(monitor, "syscalls/sys_enter_recvfrom", decodeSysEnterRecvfrom, nfs.recvfromAttemptFilters)
	registerEvent(monitor, "syscalls/sys_enter_recvmsg", decodeSysEnterRecvfrom, nfs.recvfromAttemptFilters)

	registerEvent(monitor, "syscalls/sys_exit_recvfrom", decodeSysExitRecvfrom, nfs.recvfromResultFilters)
	registerEvent(monitor, "syscalls/sys_exit_recvmsg", decodeSysExitRecvfrom, nfs.recvfromResultFilters)

	registerKprobe(monitor, KPROBE_SENDMSG_SYMBOL, KPROBE_SENDMSG_FETCHARGS, decodeSysSendto, nfs.sendtoAttemptFilters)
	registerKprobe(monitor, KPROBE_SENDTO_SYMBOL, KPROBE_SENDTO_FETCHARGS, decodeSysSendto, nfs.sendtoAttemptFilters)

	registerEvent(monitor, "syscalls/sys_exit_sendmsg", decodeSysExitSendto, nfs.sendtoResultFilters)
	registerEvent(monitor, "syscalls/sys_exit_sendto", decodeSysExitSendto, nfs.sendtoResultFilters)
}
