// Copyright (c) 2021-2025 - for information on the respective copyright owner
// see the NOTICE file and/or the repository https://github.com/carbynestack/ephemeral.
//
// SPDX-License-Identifier: Apache-2.0
//
// The original file created by the Operator SDK has been modified to add Carbyne Stack Ephemeral network controller
// logic.
package network

import (
	"context"
	"fmt"

	mpcv1alpha1 "github.com/carbynestack/ephemeral/pkg/network-controller/apis/mpc/v1alpha1"
	configtypes "github.com/carbynestack/ephemeral/pkg/types"
	clientset "github.com/knative/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"knative.dev/pkg/apis/istio/v1alpha3"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.ZapLogger(true).WithName("controller_network")

var istioGW = "test"

var podLabel = "mpc.podName"

// Add creates a new Network Controller and adds it to the PortsState. The PortsState will set fields on the Controller
// and Start it when the PortsState is Started.
func Add(mgr manager.Manager, config *configtypes.NetworkControllerConfig) error {
	return add(mgr, newReconciler(mgr, config))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, config *configtypes.NetworkControllerConfig) reconcile.Reconciler {

	c := mgr.GetConfig()
	cs := clientset.NewForConfigOrDie(c)

	return &ReconcileNetwork{client: mgr.GetClient(), scheme: mgr.GetScheme(), sharedClientSet: cs, config: config}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {

	// Add Istio resources to the scheme so we can watch them.
	v1alpha3.AddToScheme(mgr.GetScheme())

	// Create a new controller
	c, err := controller.New("network-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Network
	err = c.Watch(&source.Kind{Type: &mpcv1alpha1.Network{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource Pods and requeue the owner Network
	err = watchChildResources(c, &corev1.Service{}, &v1alpha3.VirtualService{})
	if err != nil {
		return err
	}

	return nil
}

func watchChildResources(c controller.Controller, ro ...runtime.Object) error {
	for _, o := range ro {
		err := c.Watch(&source.Kind{Type: o}, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    &mpcv1alpha1.Network{},
		})
		if err != nil {
			return err
		}
	}
	return nil
}

var _ reconcile.Reconciler = &ReconcileNetwork{}

// ReconcileNetwork reconciles a Network object
type ReconcileNetwork struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client          client.Client
	scheme          *runtime.Scheme
	sharedClientSet *clientset.Clientset
	config          *configtypes.NetworkControllerConfig
}

// Reconcile reads that state of the cluster for a Network object and makes changes based on the state read
// and what is in the Network.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNetwork) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Network")

	// Fetch the Network instance
	instance := &mpcv1alpha1.Network{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {

			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// TODO: this is a pretty hacky solution, if the pod is reconciled after an update, the label will be removed.
	// Label the pod would like to assign a service to.

	podName := instance.Labels[podLabel]
	pod := &corev1.Pod{}
	err = r.client.Get(context.TODO(), k8stypes.NamespacedName{Name: podName, Namespace: instance.Namespace}, pod)
	if err != nil {
		reqLogger.Error(err, "not able to retrieve the Pod for the service")
		return reconcile.Result{}, err
	}

	if _, ok := pod.Labels[podLabel]; !ok {
		pod.Labels[podLabel] = podName
		err = r.client.Update(context.TODO(), pod)
		if err != nil {
			reqLogger.Error(err, "not able to save the updated Pod definition")
			return reconcile.Result{}, err
		}
	}

	service := newServiceForKnativePod(instance)

	// Set Network instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, service, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if the Service already exists
	found := &corev1.Service{}
	err = r.client.Get(context.TODO(), k8stypes.NamespacedName{Name: service.Name, Namespace: service.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Service", "Service.Namespace", service.Namespace, "Service.Name", service.Name)
		err = r.client.Create(context.TODO(), service)
		if err != nil {
			reqLogger.Error(err, "not able to create a gateway")
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("Reconcile with the config:\n%+v", r.config))

	// Check if the gateway already exist, create it otherwise.

	gw := newGateway(instance, instance.Spec.Port, r.config)

	if err := controllerutil.SetControllerReference(instance, gw, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	gatewayName := gatewayName(instance.Name)
	_, err = r.sharedClientSet.NetworkingV1alpha3().Gateways(request.Namespace).Get(gatewayName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info(fmt.Sprintf("Creating a new gateway \"%s\"", gatewayName))
		_, err := r.sharedClientSet.NetworkingV1alpha3().Gateways(request.Namespace).Create(gw)
		if err != nil {
			reqLogger.Error(err, "not able to create the gateway")
			return reconcile.Result{}, err
		}
	}

	vs := newVirtualService(instance, instance.Spec.Port, gatewayName)

	if err := controllerutil.SetControllerReference(instance, vs, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	_, err = r.sharedClientSet.NetworkingV1alpha3().VirtualServices(request.Namespace).Get(vsName(request.Name), metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		reqLogger.Info("Creating a new Virtual Service", "VirtualService.Namespace", service.Namespace, "VirtualService.Name", vs.Name)
		_, err := r.sharedClientSet.NetworkingV1alpha3().VirtualServices(request.Namespace).Create(vs)
		if err != nil {
			reqLogger.Error(err, "can't create a virtual service")
			return reconcile.Result{}, err
		}
	} else if err != nil {
		reqLogger.Error(err, "error retrieving a virtual service")
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func serviceName(base string) string {
	return base + "-mpc-service"
}

func vsName(base string) string {
	return base + "-mpc-vs"
}

func gatewayName(base string) string {
	return base + "-mpc-gateway"
}

func newServer(name string, number int) v1alpha3.Server {
	return v1alpha3.Server{
		Port: v1alpha3.Port{
			Number:   number,
			Protocol: "TCP",
			Name:     name,
		},
		Hosts: []string{"*"},
	}
}

func newServiceForKnativePod(cr *mpcv1alpha1.Network) *corev1.Service {
	// Network name is the same as the pod's 'app' label.
	labels := map[string]string{
		podLabel: cr.Labels[podLabel],
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName(cr.Name),
			Namespace: cr.Namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{
				corev1.ServicePort{
					Protocol:   corev1.ProtocolTCP,
					Name:       "tcp",
					Port:       cr.Spec.TargetPort,
					TargetPort: intstr.IntOrString{IntVal: cr.Spec.TargetPort},
				},
			},
		},
	}
	return service
}

func newVirtualService(cr *mpcv1alpha1.Network, port int32, gateway string) *v1alpha3.VirtualService {
	return &v1alpha3.VirtualService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      vsName(cr.Name),
			Namespace: cr.Namespace,
			Labels:    cr.Labels,
		},
		Spec: v1alpha3.VirtualServiceSpec{
			Hosts:    []string{"*"},
			Gateways: []string{gateway},
			TCP: []v1alpha3.TCPRoute{
				v1alpha3.TCPRoute{
					Match: []v1alpha3.L4MatchAttributes{
						v1alpha3.L4MatchAttributes{
							Port: int(port)},
					},
					Route: []v1alpha3.HTTPRouteDestination{
						v1alpha3.HTTPRouteDestination{
							Destination: v1alpha3.Destination{
								Host: serviceName(cr.Name) + "." + cr.Namespace + ".svc.cluster.local",
								Port: v1alpha3.PortSelector{
									Number: uint32(cr.Spec.TargetPort),
								},
							},
							Weight: 100,
						},
					},
				},
			},
		}}
}

func newGateway(cr *mpcv1alpha1.Network, port int32, config *configtypes.NetworkControllerConfig) *v1alpha3.Gateway {
	gwlb := cr.Labels
	gwlb["mpc.gateway"] = "true"
	selectors := map[string]string{}
	selectors["istio"] = "ingressgateway"
	srv := newServer(cr.Name, int(port))
	if config.TlsEnabled {
		srv.TLS = &v1alpha3.TLSOptions{
			Mode:           v1alpha3.TLSModeMutual, // Use TLSModeMutual mode for mTLS
			CredentialName: config.TlsSecret,       // Name of the Kubernetes secret holding the certificate
		}
	}
	servers := []v1alpha3.Server{srv}
	return &v1alpha3.Gateway{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gatewayName(cr.Name),
			Namespace: cr.Namespace,
			Labels:    gwlb,
		},
		Spec: v1alpha3.GatewaySpec{
			Selector: selectors,
			Servers:  servers,
		},
	}
}
