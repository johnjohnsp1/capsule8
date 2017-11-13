package sensor

import (
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/net/context"

	"net/http"

	// Include pprof
	_ "net/http/pprof"

	"github.com/capsule8/capsule8/pkg/config"
	"github.com/capsule8/capsule8/pkg/subscription"
	"github.com/golang/glog"
)

type profilingServer struct {
	server *http.Server
}

func (p *profilingServer) Name() string {
	return "Profiling HTTP endpoint"
}

func (p *profilingServer) Serve() error {
	glog.Infof("Serving profiling HTTP endpoints on %s",
		config.Global.ProfilingAddr)

	p.server = &http.Server{
		Addr: config.Global.ProfilingAddr,
	}

	serveErr := p.server.ListenAndServe()

	glog.V(1).Info(serveErr)
	return serveErr
}

func (p *profilingServer) Stop() {
	p.server.Shutdown(context.Background())
}

// Main is the main entrypoint for the sensor
func Main() {
	if len(config.Sensor.ServerAddr) > 0 {
		s := &gRPCServer{}
		Sensor.RegisterServer(s)
	}

	if len(config.Global.ProfilingAddr) > 0 {
		s := &profilingServer{}
		Sensor.RegisterServer(s)
	}

	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		glog.Infof("Caught signal %v, stopping sensor", sig)
		Sensor.Stop()
	}()

	// Start up process monitor
	_ = subscription.ProcessMonitor()

	err := Sensor.Serve()
	if err != nil {
		glog.Error(err)
	}

	glog.Info("Exiting...")
}
