package hive

import (
	"context"
	hivev1alpha1 "github.com/openshift/hive-operator/pkg/apis/hive/v1alpha1"
	"github.com/openshift/hive-operator/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourceread"
	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	appsclientv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Hive Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileHive{
		client: mgr.GetClient(),
		scheme: mgr.GetScheme(),
    }
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("hive-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	r.(*ReconcileHive).kubeClient, err = kubernetes.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	r.(*ReconcileHive).deploymentClient, err = appsclientv1.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Hive
	err = c.Watch(&source.Kind{Type: &hivev1alpha1.Hive{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO: need to watch resources that we have created through code
	return nil
}

var _ reconcile.Reconciler = &ReconcileHive{}

// ReconcileHive reconciles a Hive object
type ReconcileHive struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	kubeClient kubernetes.Interface
	deploymentClient appsclientv1.AppsV1Interface
}

// Reconcile reads that state of the cluster for a Hive object and makes changes based on the state read
// and what is in the Hive.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHive) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	logrus.Infof("Reconciling Hive %s/%s\n", request.Namespace, request.Name)

	// Fetch the Hive instance
	instance := &hivev1alpha1.Hive{}
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

	// files is an array of strings, that are to be registered to the client
	// using resourceapply.ApplyDirectly
	files := []string{
		"deploy/config/rbac-role.yaml",
		"deploy/config/rbac_role_binding.yaml",
		"deploy/config/manager_service.yaml",
	}
	recorder := events.NewRecorder(r.kubeClient.CoreV1().Events(request.Namespace), "hive-operator", &corev1.ObjectReference{
		Name: request.Name,
		Namespace: request.Namespace,
	})
	resourceapply.ApplyDirectly(r.kubeClient, recorder, assets.Asset, files...)
	// managerDeployment is the byte array for manager_deployment.yaml, also we have to call
	// resourceapply.ApplyDeployment to register a deployment to the client
	managerDeployment := resourceread.ReadDeploymentV1OrDie(assets.MustAsset("deploy/config/manager_deployment.yaml"))
	// containers is the array of containers that manager-deployment creates.
	// It is one container for now but the code handles changing the image for multiple containers
	containers := managerDeployment.Spec.Template.Spec.Containers
	for containerIndex := 0; containerIndex < len(containers); containerIndex++ {
		containers[containerIndex].Image = instance.Spec.Image
	}
	resourceapply.ApplyDeployment(r.deploymentClient,
		recorder,
		managerDeployment,
		0,
		true)

	return reconcile.Result{}, nil
}
