//
// Copyright (c) 2021 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
package discovery

import (
	"errors"
	pb "github.com/carbynestack/ephemeral/pkg/discovery/transport/proto"
	"github.com/carbynestack/ephemeral/pkg/network-controller/apis/mpc/v1alpha1"
	clientset "github.com/carbynestack/ephemeral/pkg/network-controller/client/clientset/versioned"
	"sync"
	"time"

	cs "github.com/knative/pkg/client/clientset/versioned"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const defaultNamespace = "default"

// Networker is an interface that allows to retrieve ports and create network config for MPC apps.
type Networker interface {
	CreateNetwork(pl *pb.Player) (int32, error)
}

// NewIstioNetworker creates a new IstioNetworker
func NewIstioNetworker(logger *zap.SugaredLogger, portRange string, delCh chan string) (*IstioNetworker, error) {
	conf, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}

	networkClient := clientset.NewForConfigOrDie(conf)
	istioClient := cs.NewForConfigOrDie(conf)

	portState, err := NewPortsState(portRange, []int32{})
	if err != nil {
		return nil, err
	}

	return &IstioNetworker{
		networkingClient: networkClient,
		istioClient:      istioClient,
		ports:            portState,
		kubeConfig:       conf,
		logger:           logger,
		delCh:            delCh,
	}, nil
}

// IstioNetworker is an implementation of Networker interface which creates new networks backed by Istio configuration.
type IstioNetworker struct {
	networkingClient *clientset.Clientset
	istioClient      *cs.Clientset
	ports            *PortsState
	kubeConfig       *rest.Config
	logger           *zap.SugaredLogger
	delCh            chan string
	mux              sync.Mutex
}

// Run starts the Networker. This method initializes k8s informers and synchorinizes various caches.
// It also registers a callback which will clean up the resources after pod deletion.
func (i *IstioNetworker) Run() error {
	stopCh := make(chan struct{})
	kubeClient := kubernetes.NewForConfigOrDie(i.kubeConfig)

	kubeInformerFactory := kubeinformers.NewSharedInformerFactoryWithOptions(kubeClient, 10*time.Minute, kubeinformers.WithNamespace(defaultNamespace))

	podInformer := kubeInformerFactory.Core().V1().Pods().Informer()
	go podInformer.Run(stopCh)

	ok := cache.WaitForCacheSync(stopCh, podInformer.HasSynced)
	if !ok {
		stopCh <- struct{}{}
		return errors.New("Error syncing state of the pods")
	}
	err := i.sync()
	if err != nil {
		stopCh <- struct{}{}
		return err
	}
	i.logger.Debugf("Synced caches, starting the discovery service")
	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj interface{}, newObj interface{}) {
			pod := newObj.(*v1.Pod)
			// TODO: Make sure we delete the player info in our internal bookkeeping as well.
			if name, ok := pod.Labels[mpcPodNameLabel]; ok && pod.DeletionTimestamp != nil {
				i.deleteNetwork(name)
				i.delCh <- name
			}
		},
	})
	// Synchronize the state of the ports every 15 seconds.
	syncChan := time.Tick(15 * time.Second)
	go func() {
		for {
			select {
			case <-syncChan:
				i.logger.Debug("Synchronizing state of the ports")
				i.sync()
			case <-stopCh:
				// exit.
			}
		}
	}()
	return nil
}

// CreateNetwork creates a network in the format acceptable by the network controller.
func (i *IstioNetworker) CreateNetwork(pl *pb.Player) (int32, error) {
	port, err := i.getPort()
	if err != nil {
		i.logger.Error(err, "not able to get a free port")
		return 0, err
	}
	i.logger.Infof("Creating a new network for player %v", pl)
	lb := make(map[string]string)
	lb[mpcPodNameLabel] = pl.Pod
	network := v1alpha1.Network{
		ObjectMeta: metav1.ObjectMeta{
			Name: pl.Pod,
			// TODO: receive the namespace from the runner pod.
			Namespace: defaultNamespace,
			Labels:    lb,
		},
		// TODO: remove this 100 hack, it is a temp workaround for protobuf3.
		Spec: v1alpha1.NetworkSpec{TargetPort: BasePort + pl.Id - 100, Port: port},
	}
	_, err = i.networkingClient.MpcV1alpha1().Networks(defaultNamespace).Create(&network)
	if err != nil {
		i.logger.Error(err)
		return 0, err
	}
	return port, nil
}

// sync synchronizes its port state with k8s gateways.
func (i *IstioNetworker) sync() error {
	i.mux.Lock()
	defer i.mux.Unlock()

	usedPorts, err := i.getUsedPorts()
	err = i.ports.Sync(usedPorts)

	if err != nil {
		return err
	}

	return nil
}

// deleteNetwork executes the given callback and if the result is successful it deletes the network from k8s.
func (i *IstioNetworker) deleteNetwork(name string) error {
	err := i.networkingClient.MpcV1alpha1().Networks("default").Delete(name, &metav1.DeleteOptions{})
	if err != nil {
		i.logger.Errorf("Error deleting the network: %s", err)
		return err
	}
	return nil
}

// getUsedPorts iterates over the list of Istio gateways and collects that ports that are already in use.
func (i *IstioNetworker) getUsedPorts() ([]int32, error) {
	var usedPorts []int32
	gateways, err := i.istioClient.NetworkingV1alpha3().Gateways("default").List(metav1.ListOptions{})
	if err != nil {
		i.logger.Error(err, "unable to retrieve the gateways")
		return []int32{}, err
	}
	// i.logger.Debug("The list of received gateways", gateways.Items)
	for _, gw := range gateways.Items {
		for _, server := range gw.Spec.Servers {
			usedPorts = append(usedPorts, int32(server.Port.Number))
		}
	}
	return usedPorts, nil
}

func (i *IstioNetworker) getPort() (int32, error) {
	port, err := i.ports.GetFreePort()
	if err != nil {
		return 0, err
	}
	return port, nil
}
