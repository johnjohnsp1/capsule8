package services

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/golang/glog"
)

// Service is a service that is registered with and run by a ServiceManager
type Service interface {
	Name() string
	Serve() error
	Stop()
}

type ServiceManager struct {
	sync.Mutex

	services []Service
	stopped  bool

	// Channel that signals stopping the server
	stopChan chan struct{}
}

func NewServiceManager() *ServiceManager {
	return &ServiceManager{}
}

func (sm *ServiceManager) RegisterService(service Service) {
	sm.Lock()
	sm.services = append(sm.services, service)
	sm.Unlock()
}

func (sm *ServiceManager) Run() {
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, os.Interrupt)
	signal.Notify(sigChan, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		glog.V(1).Infof("Caught signal %v; stopping services", sig)
		sm.Stop()
	}()

	sm.Lock()
	sm.stopChan = make(chan struct{})
	services := sm.services
	sm.Unlock()

	wg := sync.WaitGroup{}

	go func() {
		<-sm.stopChan

		for _, service := range services {
			glog.V(1).Infof("Stopping service %s", service.Name())
			service.Stop()
		}

		sm.stopped = true
	}()

	glog.V(1).Info("Starting services ...")
	for _, service := range services {
		wg.Add(1)

		go func(service Service) {
			glog.V(1).Infof("Starting service %s", service.Name())
			err := service.Serve()
			glog.V(1).Infof("%s Serve(): %v", service.Name(), err)
			wg.Done()
		}(service)
	}

	// Block until goroutines have exited
	glog.V(1).Info("Server is ready")
	wg.Wait()
}

func (sm *ServiceManager) Stop() {
	sm.Lock()
	defer sm.Unlock()

	// It's ok to call Stop multiple times, so only close the stopChan if
	// the Server is running
	if !sm.stopped {
		close(sm.stopChan)
	}
}
