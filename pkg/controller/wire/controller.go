package wire

import (
	"fmt"
	"time"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	appslisters "k8s.io/client-go/listers/apps/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	clientset "github.com/argoproj/argo-events/pkg/client/clientset/versioned"
	wirescheme "github.com/argoproj/argo-events/pkg/client/clientset/versioned/scheme"
	informers "github.com/argoproj/argo-events/pkg/client/informers/externalversions"
	listers "github.com/argoproj/argo-events/pkg/client/listers/events/v1alpha1"

	eventsv1alpha1 "github.com/argoproj/argo-events/pkg/apis/events/v1alpha1"
	"github.com/argoproj/argo-events/pkg/controller/util"

	"github.com/knative/eventing/pkg/controller"
)

const controllerAgentName = "wire-controller"

const (
	// SuccessSynced is used as part of the Event 'reason' when a Wire is synced
	SuccessSynced = "Synced"
	// ErrResourceExists is used as part of the Event 'reason' when a Wire fails
	// to sync due to a Service of the same name already existing.
	ErrResourceExists = "ErrResourceExists"

	// MessageResourceExists is the message used for Events when a resource
	// fails to sync due to a Service already existing
	MessageResourceExists = "Resource %q already exists and is not managed by Wire"
	// MessageResourceSynced is the message used for an Event fired when a Wire
	// is synced successfully
	MessageResourceSynced = "Wire synced successfully"
)

const (
	// ServiceSynced is used as part of the condition reason when the wire (k8s) service is successfully created.
	ServiceSynced = "ServiceSynced"
	// ServiceError is used as part of the condition reason when the wire (k8s) service creation failed.
	ServiceError = "ServiceError"
	// DeploymentSynced is used as part of the condition reason when a wire deployment is successfully created.
	DeploymentSynced = "DeploymentSynced"
	// DeploymentError is used as part of the condition reason when a wire deployment creation failed.
	DeploymentError = "DeploymentError"
)

// Controller is the controller implementation for Wire resources
type Controller struct {
	// kubeclientset is a standard kubernetes clientset
	kubeclientset kubernetes.Interface
	// wireclientset is a clientset for our own API group
	wireclientset clientset.Interface

	deploymentsLister appslisters.DeploymentLister
	deploymentsSynced cache.InformerSynced
	servicesLister    corelisters.ServiceLister
	servicesSynced    cache.InformerSynced
	wiresLister       listers.WireLister
	wiresSynced       cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}

// NewController returns a new wire controller
func NewController(
	kubeclientset kubernetes.Interface,
	wireclientset clientset.Interface,
	restConfig *rest.Config,
	kubeInformerFactory kubeinformers.SharedInformerFactory,
	wireInformerFactory informers.SharedInformerFactory) controller.Interface {

	// obtain references to shared index informers for the Wire, Deployment and Service
	// types.
	wireInformer := wireInformerFactory.Events().V1alpha1().Wires()
	deploymentInformer := kubeInformerFactory.Apps().V1().Deployments()
	serviceInformer := kubeInformerFactory.Core().V1().Services()

	// Create event broadcaster
	wirescheme.AddToScheme(scheme.Scheme)
	log.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(log.Infof)
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	c := &Controller{
		kubeclientset:     kubeclientset,
		wireclientset:     wireclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		servicesLister:    serviceInformer.Lister(),
		servicesSynced:    serviceInformer.Informer().HasSynced,
		wiresLister:       wireInformer.Lister(),
		wiresSynced:       wireInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Wires"),
		recorder:          recorder,
	}

	log.Info("Setting up event handlers")

	// event handler for when Wire resources change
	wireInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueWire,
		UpdateFunc: func(old, new interface{}) {
			c.enqueueWire(new)
		},
	})

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newService := new.(*corev1.Service)
			oldService := old.(*corev1.Service)
			if newService.ResourceVersion == oldService.ResourceVersion {
				// Periodic resync will send update events for all known Services.
				// Two different versions of the same Service will always have different RVs.
				return
			}
			c.handleObject(new)
		},
		DeleteFunc: c.handleObject,
	})

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	log.Info("Starting Wire controller")

	// Wait for the caches to be synced before starting workers
	log.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.servicesSynced, c.wiresSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	log.Info("Starting workers")
	// Launch two workers to process Wire resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	log.Info("Started workers")
	<-stopCh
	log.Info("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Wire resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing wire '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		log.Infof("Successfully synced wire '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Wire resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Wire resource with this namespace/name
	wire, err := c.wiresLister.Wires(namespace).Get(name)
	if err != nil {
		// The Wire resource may no longer exist, in which case we stop
		// processing.
		if errors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("wire '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	var receiverService *corev1.Service
	var dispatcherDeployment, receiverDeployment *appsv1.Deployment
	var receiverServiceErr, dispatcherDeplErr, receiverDeplError error

	// Sync Receiver Service for the Wire
	receiverService, receiverServiceErr = c.syncWireReceiverService(wire)
	if receiverServiceErr != nil {
		c.updateWireStatus(wire,
			receiverService, receiverServiceErr,
			receiverDeployment, receiverDeplError,
			dispatcherDeployment, dispatcherDeplErr)
		return receiverServiceErr
	}

	// Sync Receiver Deployment for the Wire
	receiverDeployment, receiverDeplError = c.syncWireReceiverDeployment(wire)
	if receiverDeplError != nil {
		c.updateWireStatus(wire,
			receiverService, receiverServiceErr,
			receiverDeployment, receiverDeplError,
			dispatcherDeployment, dispatcherDeplErr)
		return receiverDeplError
	}

	// Sync Dispatcher Deployment for the Wire
	dispatcherDeployment, dispatcherDeplErr = c.syncWireDispatcherDeployment(wire)
	if dispatcherDeplErr != nil {
		c.updateWireStatus(wire,
			receiverService, receiverServiceErr,
			receiverDeployment, receiverDeplError,
			dispatcherDeployment, dispatcherDeplErr)
		return dispatcherDeplErr
	}

	err = c.updateWireStatus(wire,
		receiverService, receiverServiceErr,
		receiverDeployment, receiverDeplError,
		dispatcherDeployment, dispatcherDeplErr)
	if err != nil {
		return err
	}

	c.recorder.Event(wire, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
	return nil
}

func (c *Controller) updateWireStatus(
	wire *eventsv1alpha1.Wire,
	receiverService *corev1.Service, receiverServiceErr error,
	receiverDeployment *appsv1.Deployment, receiverDeploymentErr error,
	dispatcherDeployment *appsv1.Deployment, dispatcherDeploymentErr error,
) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	wireCopy := wire.DeepCopy()

	if receiverService != nil {
		wireCopy.Status.Service = &corev1.LocalObjectReference{Name: receiverService.Name}
		serviceState := util.NewWireState(eventsv1alpha1.WireServiceable, corev1.ConditionTrue, ServiceSynced, "service successfully synced")
		util.SetWireState(&wireCopy.Status, *serviceState)
	} else {
		wireCopy.Status.Service = nil
		serviceState := util.NewWireState(eventsv1alpha1.WireServiceable, corev1.ConditionFalse, ServiceError, receiverServiceErr.Error())
		util.SetWireState(&wireCopy.Status, *serviceState)
	}

	if receiverDeployment != nil {
		receiverState := util.NewWireState(eventsv1alpha1.WireReceiving, corev1.ConditionTrue, DeploymentSynced, "deployment successfully synced")
		util.SetWireState(&wireCopy.Status, *receiverState)
	} else {
		receiverState := util.NewWireState(eventsv1alpha1.WireReceiving, corev1.ConditionFalse, DeploymentError, receiverDeploymentErr.Error())
		util.SetWireState(&wireCopy.Status, *receiverState)
	}

	if dispatcherDeployment != nil {
		dispatcherState := util.NewWireState(eventsv1alpha1.WireDispatching, corev1.ConditionTrue, DeploymentSynced, "deployment successfully synced")
		util.SetWireState(&wireCopy.Status, *dispatcherState)
	} else if provisionerDeploymentErr != nil {
		provisionCondition := util.NewWireState(eventsv1alpha1.WireDispatching, corev1.ConditionFalse, DeploymentError, provisionerDeploymentErr.Error())
		util.SetWireState(&wireCopy.Status, *provisionCondition)
	} else {
		util.RemoveWireState(&wireCopy.Status, channelsv1alpha1.WireDispatching)
	}

	util.AggregateWireState(wireCopy)

	// If the CustomResourceSubresources feature gate is not enabled,
	// we must use Update instead of UpdateStatus to update the Status block of the Wire resource.
	// UpdateStatus will not allow changes to the Spec of the resource,
	// which is ideal for ensuring nothing other than resource status has been updated.
	_, err := c.wireclientset.ArgoprojV1alpha1().Wires(wire.Namespace).Update(wireCopy)
	return err
}

// enqueueWire takes a Wire resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Wire.
func (c *Controller) enqueueWire(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Wire resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Wire resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *Controller) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		log.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	log.Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Wire, we should not do anything more
		// with it.
		if ownerRef.Kind != "Wire" {
			return
		}

		wire, err := c.wiresLister.Wires(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			log.Infof("ignoring orphaned object '%s' of wire '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueWire(wire)
		return
	}
}
