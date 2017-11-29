package sensor

import (
	"fmt"
	"strings"

	api "github.com/capsule8/api/v0"

	"github.com/capsule8/capsule8/pkg/expression"
	"github.com/capsule8/capsule8/pkg/sys/perf"

	"github.com/golang/glog"
)

const (
	networkKprobeBindSymbol    = "sys_bind"
	networkKprobeBindFetchargs = "fd=%di sa_family=+0(%si):u16 sin_port=+2(%si):u16 sin_addr=+4(%si):u32 sun_path=+2(%si):string sin6_port=+2(%si):u16 sin6_addr_high=+8(%si):u64 sin6_addr_low=+16(%si):u64"

	networkKprobeConnectSymbol    = "sys_connect"
	networkKprobeConnectFetchargs = "fd=%di sa_family=+0(%si):u16 sin_port=+2(%si):u16 sin_addr=+4(%si):u32 sun_path=+2(%si):string sin6_port=+2(%si):u16 sin6_addr_high=+8(%si):u64 sin6_addr_low=+16(%si):u64"

	networkKprobeSendmsgSymbol    = "sys_sendmsg"
	networkKprobeSendmsgFetchargs = "fd=%di sa_family=+0(+0(%si)):u16 sin_port=+2(+0(%si)):u16 sin_addr=+4(+0(%si)):u32 sun_path=+2(+0(%si)):string sin6_port=+2(+0(%si)):u16 sin6_addr_high=+8(+0(%si)):u64 sin6_addr_low=+16(+0(%si)):u64"

	networkKprobeSendtoSymbol    = "sys_sendto"
	networkKprobeSendtoFetchargs = "fd=%di sa_family=+0(%r8):u16 sin_port=+2(%r8):u16 sin_addr=+4(%r8):u32 sun_path=+2(%r8):string sin6_port=+2(%r8):u16 sin6_addr_high=+8(%r8):u64 sin6_addr_low=+16(%r8):u64"
)

type networkFilter struct {
	sensor *Sensor
}

func (f *networkFilter) newNetworkEvent(eventType api.NetworkEventType, sample *perf.SampleRecord, data perf.TraceEventSampleData) *api.Event {
	// If this even contains a network address, throw away any family that
	// we do not support without doing the extra work of creating an event
	// just to throw it away
	family, have_family := data["sa_family"].(uint16)
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

	event := f.sensor.NewEventFromSample(sample, data)
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

func (f *networkFilter) decodeSysEnterAccept(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysExitAccept(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysBind(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysExitBind(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysConnect(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysExitConnect(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysEnterListen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT, sample, data)

	network := event.Event.(*api.Event_Network).Network
	network.Backlog = data["backlog"].(uint64)

	return event, nil
}

func (f *networkFilter) decodeSysExitListen(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysEnterRecvfrom(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysExitRecvfrom(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysSendto(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT, sample, data)
	return event, nil
}

func (f *networkFilter) decodeSysExitSendto(sample *perf.SampleRecord, data perf.TraceEventSampleData) (interface{}, error) {
	event := f.newNetworkEvent(api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT, sample, data)
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
	var filterString string

	if nef.FilterExpression != nil {
		expr, err := expression.NewExpression(nef.FilterExpression)
		if err != nil {
			glog.V(1).Infof("Bad network filter expression: %s", err)
			return
		}

		filterString = expr.KernelFilterString()
	}

	switch nef.Type {
	case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_ATTEMPT:
		if nfs.acceptAttemptFilters == nil {
			nfs.acceptAttemptFilters = make(map[string]int)
		}
		nfs.acceptAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_ACCEPT_RESULT:
		if nfs.acceptResultFilters == nil {
			nfs.acceptResultFilters = make(map[string]int)
		}
		nfs.acceptResultFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_ATTEMPT:
		if nfs.bindAttemptFilters == nil {
			nfs.bindAttemptFilters = make(map[string]int)
		}
		nfs.bindAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_BIND_RESULT:
		if nfs.bindResultFilters == nil {
			nfs.bindResultFilters = make(map[string]int)
		}
		nfs.bindResultFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_ATTEMPT:
		if nfs.connectAttemptFilters == nil {
			nfs.connectAttemptFilters = make(map[string]int)
		}
		nfs.connectAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_CONNECT_RESULT:
		if nfs.connectResultFilters == nil {
			nfs.connectResultFilters = make(map[string]int)
		}
		nfs.connectResultFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_ATTEMPT:
		if nfs.listenAttemptFilters == nil {
			nfs.listenAttemptFilters = make(map[string]int)
		}
		nfs.listenAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_LISTEN_RESULT:
		if nfs.listenResultFilters == nil {
			nfs.listenResultFilters = make(map[string]int)
		}
		nfs.listenResultFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_ATTEMPT:
		if nfs.recvfromAttemptFilters == nil {
			nfs.recvfromAttemptFilters = make(map[string]int)
		}
		nfs.recvfromAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_RECVFROM_RESULT:
		if nfs.recvfromResultFilters == nil {
			nfs.recvfromResultFilters = make(map[string]int)
		}
		nfs.recvfromResultFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_ATTEMPT:
		if nfs.sendtoAttemptFilters == nil {
			nfs.sendtoAttemptFilters = make(map[string]int)
		}
		nfs.sendtoAttemptFilters[filterString]++
	case api.NetworkEventType_NETWORK_EVENT_TYPE_SENDTO_RESULT:
		if nfs.sendtoResultFilters == nil {
			nfs.sendtoResultFilters = make(map[string]int)
		}
		nfs.sendtoResultFilters[filterString]++
	}
}

func fullFilterString(filters map[string]int) (string, bool) {
	if filters == nil {
		return "", false
	}
	parts := make([]string, 0, len(filters))
	for f, count := range filters {
		if count <= 0 {
			continue
		}
		if f == "" {
			return "", true
		}
		parts = append(parts, fmt.Sprintf("(%s)", f))
	}
	if len(parts) > 0 {
		return strings.Join(parts, " || "), true
	}
	return "", false
}

func registerEvent(monitor *perf.EventMonitor, name string, fn perf.TraceEventDecoderFn, filters map[string]int) {
	f, active := fullFilterString(filters)
	if !active {
		return
	}

	err := monitor.RegisterEvent(name, fn, f, nil)
	if err != nil {
		glog.Warningf("Could not register tracepoint %s", name)
	}
}

func registerKprobe(monitor *perf.EventMonitor, symbol string, fetchargs string, fn perf.TraceEventDecoderFn, filters map[string]int) {
	f, active := fullFilterString(filters)
	if !active {
		return
	}

	name := perf.UniqueProbeName("capsule8", symbol)
	_, err := monitor.RegisterKprobe(name, symbol, false, fetchargs, fn, f, nil)
	if err != nil {
		glog.Warningf("Could not register network kprobe %s", symbol)
	}
}

func registerNetworkEvents(monitor *perf.EventMonitor, sensor *Sensor, events []*api.NetworkEventFilter) {
	nfs := networkFilterSet{}
	for _, nef := range events {
		nfs.add(nef)
	}

	f := networkFilter{
		sensor: sensor,
	}

	registerEvent(monitor, "syscalls/sys_enter_accept", f.decodeSysEnterAccept, nfs.acceptAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_accept", f.decodeSysExitAccept, nfs.acceptResultFilters)

	registerKprobe(monitor, networkKprobeBindSymbol, networkKprobeBindFetchargs, f.decodeSysBind, nfs.bindAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_bind", f.decodeSysExitBind, nfs.bindResultFilters)

	registerKprobe(monitor, networkKprobeConnectSymbol, networkKprobeConnectFetchargs, f.decodeSysConnect, nfs.connectAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_connect", f.decodeSysExitConnect, nfs.connectResultFilters)

	registerEvent(monitor, "syscalls/sys_enter_listen", f.decodeSysEnterListen, nfs.listenAttemptFilters)
	registerEvent(monitor, "syscalls/sys_exit_listen", f.decodeSysExitListen, nfs.listenResultFilters)

	// There are two additional system calls added in Linux 3.0 that are of
	// interest, but there's no way to get all of the data without eBPF
	// support, so don't bother with them for now.

	registerEvent(monitor, "syscalls/sys_enter_recvfrom", f.decodeSysEnterRecvfrom, nfs.recvfromAttemptFilters)
	registerEvent(monitor, "syscalls/sys_enter_recvmsg", f.decodeSysEnterRecvfrom, nfs.recvfromAttemptFilters)

	registerEvent(monitor, "syscalls/sys_exit_recvfrom", f.decodeSysExitRecvfrom, nfs.recvfromResultFilters)
	registerEvent(monitor, "syscalls/sys_exit_recvmsg", f.decodeSysExitRecvfrom, nfs.recvfromResultFilters)

	registerKprobe(monitor, networkKprobeSendmsgSymbol, networkKprobeSendmsgFetchargs, f.decodeSysSendto, nfs.sendtoAttemptFilters)
	registerKprobe(monitor, networkKprobeSendtoSymbol, networkKprobeSendtoFetchargs, f.decodeSysSendto, nfs.sendtoAttemptFilters)

	registerEvent(monitor, "syscalls/sys_exit_sendmsg", f.decodeSysExitSendto, nfs.sendtoResultFilters)
	registerEvent(monitor, "syscalls/sys_exit_sendto", f.decodeSysExitSendto, nfs.sendtoResultFilters)
}
