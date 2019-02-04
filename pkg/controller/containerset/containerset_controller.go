/*
Copyright 2019 Masudur Rahman.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containerset

import (
	"context"
	"fmt"
	"k8s.io/client-go/tools/record"
	"reflect"

	workloadsv1beta1 "github.com/masudur-rahman/CRD-Controller-kubebuilder/pkg/apis/workloads/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ContainerSet Controller and adds it to the Manager with default RBAC. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileContainerSet{Client: mgr.GetClient(), scheme: mgr.GetScheme(), recorder: mgr.GetRecorder("containerset-controller")}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("containerset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to ContainerSet
	err = c.Watch(&source.Kind{Type: &workloadsv1beta1.ContainerSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TO-DO:(user): Modify this to be the types you create
	// Uncomment watch a Deployment created by ContainerSet - change this for objects you create
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &workloadsv1beta1.ContainerSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

var _ reconcile.Reconciler = &ReconcileContainerSet{}

// ReconcileContainerSet reconciles a ContainerSet object
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch;delete
type ReconcileContainerSet struct {
	client.Client
	scheme   *runtime.Scheme
	recorder record.EventRecorder
}

// Reconcile reads that state of the cluster for a ContainerSet object and makes changes based on the state read
// and what is in the ContainerSet.Spec
// TO-DO(user): Modify this Reconcile function to implement your Controller logic.  The scaffolding writes
// a Deployment as an example
// Automatically generate RBAC rules to allow the Controller to read and write Deployments
// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=apps,resources=deployments/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=workloads.k8s.io,resources=containersets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=workloads.k8s.io,resources=containersets/status,verbs=get;update;patch
func (r *ReconcileContainerSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	// Fetch the ContainerSet instance
	instance := &workloadsv1beta1.ContainerSet{}
	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// ---------------------------------------------------------------------------------------------
	myFinalizerName := "storage.finalizers.k8s.io"

	if instance.ObjectMeta.DeletionTimestamp.IsZero() {
		if !containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {
			instance.ObjectMeta.Finalizers = append(instance.ObjectMeta.Finalizers, myFinalizerName)

			if err := r.Update(context.Background(), instance); err != nil {
				return reconcile.Result{Requeue: true}, nil
			}
		}
	} else {
		if containsString(instance.ObjectMeta.Finalizers, myFinalizerName) {
			if err := r.deleteExternalDependency(instance); err != nil {
				return reconcile.Result{}, err
			}
		}

		instance.ObjectMeta.Finalizers = removeString(instance.ObjectMeta.Finalizers, myFinalizerName)

		if err := r.Update(context.Background(), instance); err != nil {
			return reconcile.Result{Requeue: true}, nil
		}
	}
	// -----------------------------------------------------------------------------------------------

	// ----------------------- Deployment from Instance ----------------------------------------------
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-deployment",
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &instance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"deployment": instance.Name + "-deployment"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"deployment": instance.Name + "-deployment"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  instance.Name,
							Image: instance.Spec.Image,
						},
					},
				},
			},
		},
	}
	// ------------------------------------------------------------------------------------
	if err := controllerutil.SetControllerReference(instance, deploy, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// ---------------------- Deployment Creation if not exists ---------------------------
	found := &appsv1.Deployment{}
	err = r.Get(context.TODO(), types.NamespacedName{Name: deploy.Name, Namespace: deploy.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		log.Info("Creating Deployment", "namespace", deploy.Namespace, "name", deploy.Name)
		err = r.Create(context.TODO(), deploy)

		instance.Status.HealthyReplicas = deploy.Status.AvailableReplicas
		if err := r.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{Requeue: true}, nil
		}

		fmt.Println("\nHealthy Replicas: ", instance.Status.HealthyReplicas)

		// Event recorder
		r.recorder.Event(instance, "Normal", "Created", fmt.Sprintf("Created deployment %s %s", deploy.Namespace, deploy.Name))

		return reconcile.Result{}, err
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// ------------------------ Deployment Updation if Spec is not changed ---------------------
	if !reflect.DeepEqual(deploy.Spec, found.Spec) {
		found.Spec = deploy.Spec
		log.Info("Updating Deployment", "namespace", deploy.Namespace, "name", deploy.Name)
		err = r.Update(context.TODO(), found)
		if err != nil {
			return reconcile.Result{}, err
		}
		instance.Status.HealthyReplicas = found.Status.AvailableReplicas

		if err := r.Status().Update(context.TODO(), instance); err != nil {
			return reconcile.Result{}, err
		}

		fmt.Println("\nHealthy Replicas: ", instance.Status.HealthyReplicas)

		// Event recorder
		r.recorder.Event(instance, "Normal", "Updated", fmt.Sprintf("Updated deployment %s %s", deploy.Namespace, deploy.Name))
	}
	return reconcile.Result{}, nil
}

func (r *ReconcileContainerSet) deleteExternalDependency(instance *workloadsv1beta1.ContainerSet) error {
	fmt.Println("deleting the external dependencies")

	return nil
}

// Helper functions

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) (result []string) {
	for _, item := range slice {
		if item == s {
			continue
		}
		result = append(result, item)
	}
	return
}
