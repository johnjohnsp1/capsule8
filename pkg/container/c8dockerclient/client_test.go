//c8dockerclient unittests
//This is mostly using the fake server to replay sessions from different docker versions
package c8dockerclient

import (
	"bytes"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

var unixSockPath = "./dockersock_sock.sock"

//FakeDockerServer serves as a mock docker server. It is loaded with recorded responses
//from an actual docker session at the version specified. We use a serve mux to
//encapsulate the responses
type FakeDockerServer struct{}

func (f *FakeDockerServer) listenAndServe(t *testing.T, expected chan []byte) (err error) {
	var buf [1024]byte

	l, err := net.Listen("unix", unixSockPath)
	if err != nil {
		t.Log(err)
		return err
	}
	defer os.Remove(unixSockPath)

	msg := <-expected
	resp := &http.Response{Body: ioutil.NopCloser(bytes.NewBuffer(msg))}
	resp.StatusCode = 200
	resp.ProtoMajor = 1
	resp.ProtoMinor = 1
	resp.Close = false
	resp.Header = make(http.Header)
	resp.Header.Add("Content-Type", "application/json")
	resp.TransferEncoding = []string{"chunked"}
	sock, err := l.Accept()

	if err != nil {
		t.Log(err)
		return err
	}

	sock.Read(buf[:])
	w := bytes.NewBuffer(nil)
	resp.Write(w)
	sock.Write(w.Bytes())
	return nil
}

func DockerTestSetup(t *testing.T, filename string) (cli *Client) {
	f := FakeDockerServer{}
	expectedMsg := make(chan []byte)
	go f.listenAndServe(t, expectedMsg)

	msg, err := ioutil.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}

	cli = &Client{SocketPath: unixSockPath}

	//important do not move the order of this channel send as we're using
	//it as an ad hoc means of coordinating the server
	expectedMsg <- msg
	return cli
}

func TestC8DockerClientDocker1_13_0_info(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/info_v1.13.0.json")
	dockerInfo, err := cli.DockerInfo()

	if err != nil {
		t.Fatal(err)
	}
	expectedID := "L7PS:OTTR:OX2G:47LW:NESJ:V4HB:RJMF:VEZP:3KHK:WPP7:6SKA:TXJI"

	if dockerInfo.DockerID != expectedID {
		t.Fatalf("Incorrect DockerID returned expected (%s) got (%s)", expectedID,
			dockerInfo.DockerID)
	}

	if dockerInfo.DockerVersion != "1.13.0" {
		t.Fatalf("Incorrect DockerVersion returned expected (1.13.0) got %s",
			dockerInfo.DockerVersion)
	}
	if dockerInfo.Architecture != "x86_64" {
		t.Fatalf("Incorrect Architecture in dockerInfo expected x86_64 got %s",
			dockerInfo.Architecture)
	}
	if dockerInfo.KernelVersion != "4.9.4-moby" {
		t.Fatalf("Incorrect dockerInfo.KernelVersion expected 4.9.4-moby got %s",
			dockerInfo.KernelVersion)
	}
	if dockerInfo.OS != "Alpine Linux v3.5" {
		t.Fatalf("Incorrect dockerInfo.OS expected Alpine Linux v3.5 got %s",
			dockerInfo.OS)
	}

}

func TestC8DockerClientDocker1_13_0_listContainers(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/listcontainers_v1.13.0.json")
	containers, err := cli.ListContainers()
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("Incorrect number of containers from listContainer expected 1 got %s",
			len(containers))
	}

	container := containers[0]
	if container.ContainerID != "0eb9e65780a5a1523cbd85017204591d37bb84d22f7b2d765d159b52909e3111" {
		t.Fatalf("Incorrect containerID returns from listContainers got %s", container.ContainerID)
	}
}

func TestC8DockerClientDocker1_12_2_info(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/info_v1.12.2.json")
	dockerInfo, err := cli.DockerInfo()
	if err != nil {
		t.Fatal(err)
	}
	if dockerInfo.DockerVersion != "1.12.2" {
		t.Fatal("Incorrect dockerInfo.DockerVersion " + dockerInfo.DockerVersion)
	}
	if dockerInfo.DockerID != "CRL5:PPUT:HMYO:C2DP:FQFI:6K2N:GZQ7:476M:GYVG:NUB2:XOJB:5QYO" {
		t.Fatal("Incorrect dockerInfo.DockerID " + dockerInfo.DockerID)
	}

	if dockerInfo.KernelVersion != "4.6.4-040604-generic" {
		t.Fatalf("Incorrect dockerInfo.KernelVersion expected 4.6.4-040604-generic got %s",
			dockerInfo.KernelVersion)
	}

	if dockerInfo.Architecture != "x86_64" {
		t.Fatalf("Incorrect Architecture in dockerInfo expected x86_64 got %s",
			dockerInfo.Architecture)
	}
	if dockerInfo.ContainersRunning != 1 {
		t.Fatalf("Incorrect dockerInfo.ContainersRunning value %d",
			dockerInfo.ContainersRunning)
	}
}

func TestC8DockerClientDocker1_13_3_inspectContainer(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectContainer_v1.13.0.json")
	containerInfo, err := cli.InspectContainer("5dbf0875b45bf27db5858bbfcc2e4d1db5747a5b65abc824f175281cd53b506e")
	if err != nil {
		t.Fatal(err)
	}
	if containerInfo.ContainerID != "5dbf0875b45bf27db5858bbfcc2e4d1db5747a5b65abc824f175281cd53b506e" {
		t.Fatal("Incorrect containerInfo.containerID: " + containerInfo.ContainerID)
	}

	if containerInfo.ImageID != "sha256:baa5d63471ead618ff91ddfacf1e2c81bf0612bfeb1daf00eb0843a41fbfade3" {
		t.Fatal("Incorrect ImageID: " + containerInfo.ImageID)
	}

	if containerInfo.Name != "/gallant_haibt" {
		t.Fatal("Incorrect containerInfo.Name: " + containerInfo.Name)
	}

	for _, v := range containerInfo.Arguments {
		if v != "" {
			t.Fatal("Incorrect containerInfo.Arguments expected \"\"")
		}
	}

	if containerInfo.State.ProcessID != 7749 {
		t.Fatalf("Incorrect state ProcessID %d", containerInfo.State.ProcessID)
	}

	if containerInfo.State.StartTime.Format(time.RFC3339Nano) != "2017-01-24T18:31:08.303257829Z" {
		t.Fatal("Incorrect TicontainerInfo.State.StartTime")
	}
	//the network
	if containerInfo.NetworkSettings.IPAddress != "172.17.0.2" {
		t.Fatal("Incorrect containerInfo.NetworkSettings.IPAddress")
	}

	if containerInfo.NetworkSettings.IPPrefixLen != 16 {
		t.Fatal("Incorrect containerInfo.Network.IPPrefixLen")
	}

	if len(containerInfo.NetworkSettings.Networks) > 1 {
		t.Fatal("Too many networks in containerInfo.NetworkSettings.Networks")
	}

	network, ok := containerInfo.NetworkSettings.Networks["bridge"]
	if !ok {
		t.Fatal("Missing expected network")
	}

	if network.IPAddress != "172.17.0.2" {
		t.Fatal("network.IPAddress " + network.IPAddress)
	}
}

func TestC8DockerClientDocker1_12_2_listContainers(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/listcontainers_v1.12.2.json")
	containers, err := cli.ListContainers()
	if err != nil {
		t.Fatal(err)
	}
	if len(containers) != 1 {
		t.Fatalf("Incorrect number of containers from listContainer expected 1 got %s",
			len(containers))
	}

	container := containers[0]
	if container.ContainerID != "5bef2b3f31a59a5aa07c6edc6f3853020d31f377e6d25f79ecc3c3ffb393e83d" {
		t.Fatal("Incorrect containerID returns from listContainers")
	}
}

func TestC8DockerClientDocker1_12_2_inspectContainer(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectContainer_v1.12.2.json")
	containerInfo, err := cli.InspectContainer("5bef2b3f31a59a5aa07c6edc6f3853020d31f377e6d25f79ecc3c3ffb393e83d")
	if err != nil {
		t.Fatal(err)
	}
	if containerInfo.ContainerID != "5bef2b3f31a59a5aa07c6edc6f3853020d31f377e6d25f79ecc3c3ffb393e83d" {
		t.Fatal("Incorrect containerInfo.containerID: " + containerInfo.ContainerID)
	}

	if containerInfo.ImageID != "sha256:88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd" {
		t.Fatal("Incorrect ImageID: " + containerInfo.ImageID)
	}

	if containerInfo.Name != "/thirsty_borg" {
		t.Fatal("Incorrect containerInfo.Name: " + containerInfo.Name)
	}

	for _, v := range containerInfo.Arguments {
		if v != "" {
			t.Fatal("Incorrect containerInfo.Arguments expected \"\"")
		}
	}

	if containerInfo.State.ProcessID != 6751 {
		t.Fatalf("Incorrect state ProcessID %d", containerInfo.State.ProcessID)
	}

	if containerInfo.State.StartTime.Format(time.RFC3339Nano) != "2017-01-24T04:10:36.476134615Z" {
		t.Fatal("Incorrect TicontainerInfo.State.StartTime " + containerInfo.State.StartTime.Format(time.RFC3339Nano))
	}
	//the network
	if containerInfo.NetworkSettings.IPAddress != "172.17.0.2" {
		t.Fatal("Incorrect containerInfo.NtworkSettings.IPAddress")
	}

	if len(containerInfo.NetworkSettings.Networks) != 1 {
		t.Fatal("Incorrect number of networks in containerInfo.NetworkSettings.Networks")
	}

	network, ok := containerInfo.NetworkSettings.Networks["bridge"]
	if !ok {
		t.Fatal("Missing expected network")
	}

	if network.IPAddress != "172.17.0.2" {
		t.Fatal("network.IPAddress " + network.IPAddress)
	}

	if len(containerInfo.Mounts) != 0 {
		t.Fatal("Incorrect number of Mounts found in containerInfo")
	}
}

//This tests that containerTop works with output from Docker 1.12.2 on Linux.
func TestContainerTop_v1_12_2_Linux(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/containerTop_v1.12.2.json")
	//this can be any id as we've stubbed out the response
	processes, err := cli.ContainerTop("XXX")
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 {
		t.Fatalf("Expected only one process found %d", len(processes))
	}

	process := processes[0]
	if process.ProcessID != 6751 {
		t.Fatalf("Incorrect ProcessID: %d", process.ProcessID)
	}
	if process.ParentProcessID != 6736 {
		t.Fatalf("Incorrect ParentProcessID: %d", process.ParentProcessID)
	}

	if process.CGroup != 0 {
		t.Fatal("Incorrect cGroup")
	}

	if process.User != "root" {
		t.Fatalf("Incorrect username %s", process.User)
	}
	if process.Command != "/bin/sh" {
		t.Fatalf("Incorrect command %s", process.Command)
	}
}

//This tests that containerTop works with output from Docker 1.13.0 on Linux.
func TestContainerTop_v1_13_0_Linux(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/containerTop_v1.13.0_linux.json")
	//this can be any id as we've stubbed out the response
	processes, err := cli.ContainerTop("XXX")
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 {
		t.Fatalf("Expected only one process found %d", len(processes))
	}

	process := processes[0]
	if process.ProcessID != 4331 {
		t.Fatalf("Incorrect ProcessID: %d", process.ProcessID)
	}
	if process.ParentProcessID != 4313 {
		t.Fatalf("Incorrect ParentProcessID: %d", process.ParentProcessID)
	}

	if process.CGroup != 0 {
		t.Fatal("Incorrect cGroup")
	}

	if process.User != "root" {
		t.Fatalf("Incorrect username %s", process.User)
	}
	if process.Command != "/bin/sh" {
		t.Fatalf("Incorrect command %s", process.Command)
	}
}

//This tests that containerTop works with output from Docker 1.13.0 on Docker for Mac.
func TestContainerTop_v1_13_0_Mac(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/containerTop_v1.13.0_mac.json")
	//this can be any id as we've stubbed out the response
	processes, err := cli.ContainerTop("XXX")
	if err != nil {
		t.Fatal(err)
	}
	if len(processes) != 1 {
		t.Fatalf("Expected only one process found %d", len(processes))
	}

	process := processes[0]
	if process.ProcessID != 8350 {
		t.Fatalf("Incorrect ProcessID: %d", process.ProcessID)
	}
	if process.ParentProcessID != 0 {
		t.Fatalf("Incorrect ParentProcessID: %d", process.ParentProcessID)
	}

	if process.CGroup != 0 {
		t.Fatal("Incorrect cGroup")
	}

	if process.User != "root" {
		t.Fatalf("Incorrect username %s", process.User)
	}
	if process.Command != "/bin/sh" {
		t.Fatalf("Incorrect command %s", process.Command)
	}
}

func TestInspectImage_v1_12_2(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectImage_v1_12_2.json")
	image, err := cli.InspectImage("88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd")
	if err != nil {
		t.Fatal(err)
	}
	if image.ImageID != "sha256:88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd" {
		t.Fatalf("expected ImageID of (%s)\n\t got %s",
			"88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd",
			image.ImageID)
	}
	if image.ParentID != "" {
		t.Fatalf("Unexpected ParentID: %s", image.ParentID)
	}

	if len(image.RepoTags) != 1 {
		t.Fatalf("Incorrect number of RepoTags %d", len(image.RepoTags))
	}

	for _, tag := range image.RepoTags {
		if tag != "alpine:latest" {
			t.Fatalf("Incorrect tag in RepoTags")
		}
		break
	}

	if image.Layers.Type != "layers" {
		t.Fatalf("Incorrect type (%s) for layers in RootFSLayer", image.Layers.Type)
	}

	if len(image.Layers.Layers) != 1 {
		t.Fatalf("Incorrect number of layers returned")
	}

	for _, layer := range image.Layers.Layers {
		if layer != "sha256:60ab55d3379d47c1ba6b6225d59d10e1f52096ee9d5c816e42c635ccc57a5a2b" {
			t.Fatalf("Unexpected layer %s", layer)
		}
	}
}

//This is essentially the same as the last but there are some minor differences in the file
func TestInspectImage_v1_13_0(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectImage_v1_13_0.json")
	image, err := cli.InspectImage("88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd")
	if err != nil {
		t.Fatal(err)
	}
	if image.ImageID != "sha256:88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd" {
		t.Fatalf("expected ImageID of (%s)\n\t got %s",
			"88e169ea8f46ff0d0df784b1b254a15ecfaf045aee1856dca1ec242fdd231ddd",
			image.ImageID)
	}
	if image.ParentID != "" {
		t.Fatalf("Unexpected ParentID: %s", image.ParentID)
	}

	if len(image.RepoTags) != 1 {
		t.Fatalf("Incorrect number of RepoTags %d", len(image.RepoTags))
	}

	for _, tag := range image.RepoTags {
		if tag != "alpine:latest" {
			t.Fatalf("Incorrect tag in RepoTags")
		}
		break
	}

	if image.Layers.Type != "layers" {
		t.Fatalf("Incorrect type (%s) for layers in RootFSLayer", image.Layers.Type)
	}

	if len(image.Layers.Layers) != 1 {
		t.Fatalf("Incorrect number of layers returned")
	}

	for _, layer := range image.Layers.Layers {
		if layer != "sha256:60ab55d3379d47c1ba6b6225d59d10e1f52096ee9d5c816e42c635ccc57a5a2b" {
			t.Fatalf("Unexpected layer %s", layer)
		}
	}
}

func TestContainerInspectWithMount(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectContainerWithMount_v1.13.0.json")
	containerInfo, err := cli.InspectContainer("9c6c568a87f2360664df65988b5e6232e3ad2a7abba71224c6fb40e1e5244053")
	if err != nil {
		t.Fatal(err)
	}

	if len(containerInfo.Mounts) != 1 {
		t.Fatalf("Unexpected number of mounts %d", len(containerInfo.Mounts))
	}
	mount := containerInfo.Mounts[0]
	if mount.Type != MOUNT_TYPE_BIND {
		t.Fatalf("Unexpected mount type %s", mount.Type)
	}

	if mount.Source != "/tmp" {
		t.Fatalf("Unexpected mount source: %s", mount.Source)
	}

	if mount.Destination != "/tmp/outside_tmp" {
		t.Fatalf(mount.Destination)
	}

	if mount.ReadWrite != true {
		t.Fatal("Expected mount to be ReadWrite")
	}

	if mount.Mode != "" {
		t.Fatalf("Unexpected value for mount.Mode %s", mount.Mode)
	}

	if mount.Propagation != "" {
		t.Fatalf("Unexpected mount.Propagation value %s", mount.Propagation)
	}
	if len(containerInfo.NetworkSettings.Ports) != 0 {
		t.Fatalf("Discovered forwarded ports when none were supposed to be %v",
			containerInfo.NetworkSettings.Ports)
	}
}

func TestContainerDiffV1_12_2(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/diff_v1.12.2.json")
	//this is ok because we've stubbed out the response
	containerDiff, err := cli.ContainerDiff("XXX")
	if err != nil {
		t.Fatal(err)
	}

	if len(containerDiff) != 5 {
		t.Fatalf("Expected 5 entries in ContainerDiff changes got %d,\n %v",
			len(containerDiff), containerDiff)
	}

	expectedValues := []struct {
		Kind uint8
		Path string
	}{
		{DIFF_MODIFIED, "/bin"},      // 0 stands for change in directory / file
		{DIFF_DELETED, "/bin/touch"}, // 2 stands for removed / deleted file or directory
		{DIFF_ADDED, "/foo"},         // 1 stands for directory / file added
		{DIFF_MODIFIED, "/root"},
		{DIFF_ADDED, "/root/.ash_history"},
	}
	for i := 0; i < 5; i++ {
		if containerDiff[i].Kind != expectedValues[i].Kind {
			t.Fatalf("Entry Kind %d does not match expected %v got %v", i,
				expectedValues[i].Kind, containerDiff[i].Kind)
		}
		if containerDiff[i].Path != expectedValues[i].Path {
			t.Fatalf("Entry Path %d does not match expected %v got %v", i,
				expectedValues[i].Path, containerDiff[i].Path)
		}
	}
}

func TestContainerInspectWithPortsForwarded(t *testing.T) {
	cli := DockerTestSetup(t, "./testdata/inspectContainerWithPortsForwarded_1_13_0.json")
	//this is ok because we've stubbed out the response
	containerInfo, err := cli.InspectContainer("XXX")
	if err != nil {
		t.Fatal(err)
	}
	if len(containerInfo.NetworkSettings.Ports) != 1 {
		t.Fatalf("Found incorrect number of forwarded ports %v",
			containerInfo.NetworkSettings.Ports)
	}
loop:
	for k, ports := range containerInfo.NetworkSettings.Ports {
		if k != "22/tcp" {
			t.Fatalf("Incorrect port forwarded %s", k)
		}
		if len(ports) != 1 {
			t.Fatalf("Incorrect number of ports forwarded entries %d", len(ports))
		}

		for _, portForward := range ports {
			if portForward.HostIP != "0.0.0.0" {
				t.Fatalf("Incorrect HostIP in portForward %s", portForward.HostIP)
			}

			if portForward.HostPort != "8888" {
				t.Fatalf("Incorrect HostPort in portForward %s", portForward.HostPort)
			}
		}
		break loop
	}
}
