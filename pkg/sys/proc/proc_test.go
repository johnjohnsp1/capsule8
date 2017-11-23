package proc

import (
	"testing"
)

var (
	// This data was taken from actual /proc/PID/stat files.
	statParseTestData = []statParseData{
		{
			statFile:   "4018 (bash) S 4011 4018 4018 34834 7516 4194304 8082 41779 1 85 33 7 115 329 20 0 1 0 8810 24444928 1667 18446744073709551615 4194304 5192876 140734725904528 140734725903192 140515966087290 0 65536 3670020 1266777851 1 0 0 17 3 0 0 1 0 0 7290352 7326856 31535104 140734725912793 140734725912798 140734725912798 140734725914606 0\n",
			pid:        4018,
			comm:       "bash",
			ppid:       4011,
			startTime:  8810,
			startStack: 140734725904528,
		},
		{
			statFile:   "899 (rs:main Q:Reg) S 1 828 828 0 -1 1077936192 720 0 5 0 374 450 0 0 20 0 4 0 512 262553600 2530 18446744073709551615 1 1 0 0 0 0 2146172671 16781830 1132545 0 0 0 -1 1 0 0 9 0 0 0 0 0 0 0 0 0 0\n",
			pid:        899,
			comm:       "rs:main Q:Reg",
			ppid:       1,
			startTime:  512,
			startStack: 0,
		},
		{
			statFile:   "25663 (a b) S 4090 25663 4090 34833 25831 4194304 112 0 0 0 0 0 0 0 20 0 1 0 2591294 4616192 191 18446744073709551615 93931362930688 93931363074588 140721799437360 140721799436056 139765395259690 0 0 0 65538 1 0 0 17 0 0 0 0 0 0 93931365175144 93931365179936 93931378774016 140721799438724 140721799438752 140721799438752 140721799442404 0\n",
			pid:        25663,
			comm:       "a b",
			ppid:       4090,
			startTime:  2591294,
			startStack: 140721799437360,
		},
		{
			statFile:   "25666 ((c) S 4090 25666 4090 34833 25831 4194304 111 0 0 0 0 0 0 0 20 0 1 0 2591294 4616192 197 18446744073709551615 94586441084928 94586441228828 140737160769408 140737160768104 140708343980330 0 0 0 65538 1 0 0 17 3 0 0 0 0 0 94586443329384 94586443334176 94586462375936 140737160774023 140737160774050 140737160774050 140737160777701 0\n",
			pid:        25666,
			comm:       "(c",
			ppid:       4090,
			startTime:  2591294,
			startStack: 140737160769408,
		},
		{
			statFile:   "25669 (d)) S 4090 25669 4090 34833 25831 4194304 114 0 0 0 0 0 0 0 20 0 1 0 2591295 4616192 201 18446744073709551615 93918460887040 93918461030940 140727364187808 140727364186504 140658074984746 0 0 0 65538 1 0 0 17 3 0 0 0 0 0 93918463131496 93918463136288 93918473555968 140727364190599 140727364190626 140727364190626 140727364194277 0\n",
			pid:        25669,
			comm:       "d)",
			ppid:       4090,
			startTime:  2591295,
			startStack: 140727364187808,
		},
		{
			statFile:   "25672 (((e))) S 4090 25672 4090 34833 25831 4194304 114 0 0 0 0 0 0 0 20 0 1 0 2591295 4616192 178 18446744073709551615 94113212719104 94113212863004 140724070346384 140724070345080 140031172235562 0 0 0 65538 1 0 0 17 0 0 0 0 0 0 94113214963560 94113214968352 94113226104832 140724070355326 140724070355356 140724070355356 140724070359010 0\n",
			pid:        25672,
			comm:       "((e))",
			ppid:       4090,
			startTime:  2591295,
			startStack: 140724070346384,
		},
		{
			statFile:   "25675 ( f  ) S 4090 25675 4090 34833 25831 4194304 111 0 0 0 0 0 0 0 20 0 1 0 2591295 4616192 191 18446744073709551615 94829034725376 94829034869276 140737237421792 140737237420488 139937926709546 0 0 0 65538 1 0 0 17 2 0 0 0 0 0 94829036969832 94829036974624 94829068091392 140737237426561 140737237426590 140737237426590 140737237430243 0\n",
			pid:        25675,
			comm:       " f  ",
			ppid:       4090,
			startTime:  2591295,
			startStack: 140737237421792,
		},
	}
)

type statParseData struct {
	statFile   string
	pid        int
	comm       string
	ppid       int
	startTime  uint64
	startStack uint64
}

// TestStatParse tests the parsing of /proc/PID/stat files.
func TestStatParse(t *testing.T) {
	for _, td := range statParseTestData {
		ps := &ProcessStatus{statFields: statFields(td.statFile)}

		if ps.PID() != td.pid {
			t.Errorf("For proc.(*ProcessStatus)PID(), want %d, got %d\n", td.pid, ps.PID())
		}
		if ps.Command() != td.comm {
			t.Errorf("For proc.(*ProcessStatus)Command(), want \"%s\", got \"%s\"\n", td.comm, ps.Command())
		}
		if ps.ParentPID() != td.ppid {
			t.Errorf("For proc.(*ProcessStatus)ParentPID(), want %d, got %d\n", td.ppid, ps.ParentPID())
		}
		if ps.StartTime() != td.startTime {
			t.Errorf("For proc.(*ProcessStatus)startTime(), want %d, got %d\n", td.startTime, ps.StartTime())
		}
		if ps.StartStack() != td.startStack {
			t.Errorf("For proc.(*ProcessStatus)StartStack(), want %d, got %d\n", td.startStack, ps.StartStack())
		}
	}
}

var cgroupTests = []struct {
	cgroupFile  string
	containerID string
}{
	{
		`13:pids:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
12:hugetlb:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
11:net_prio:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
10:perf_event:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
9:net_cls:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
8:freezer:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
7:devices:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
6:memory:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
5:blkio:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
4:cpuacct:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
3:cpu:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
2:cpuset:/docker/e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4
1:name=openrc:/docker
0::/docker
`, "e871ee9a818bab3222c94efe196e8555cb372676e96fea847a609c2d39e187a4"},
	{`10:hugetlb:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
9:perf_event:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
8:blkio:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
7:net_cls:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
6:freezer:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
5:devices:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
4:memory:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
3:cpuacct,cpu:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
2:cpuset:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
1:name=systemd:/system.slice/docker-47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81.scope
`, "47490dda5cd7e409e7bf04a8b291f87f15031090a955dac9ceed6a2160474d81"},
	{`9:net_cls:/
8:devices:/user.slice
7:cpu,cpuacct:/user.slice
6:pids:/user.slice/user-1000.slice/session-5.scope
5:memory:/user.slice
4:cpuset:/
3:blkio:/user.slice
2:freezer:/
1:name=systemd:/user.slice/user-1000.slice/session-5.scope
0::/user.slice/user-1000.slice/session-5.scope
`, ""},
}

func TestCgroupParse(t *testing.T) {
	for _, tc := range cgroupTests {
		cgroups := parseProcPidCgroup([]byte(tc.cgroupFile))
		cID := containerIDFromCgroups(cgroups)
		if cID != tc.containerID {
			t.Errorf("Expected container ID %s, got %s",
				tc.containerID, cID)
		}
	}
}
