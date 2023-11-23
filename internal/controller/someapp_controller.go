/*
Copyright 2023 changqings.

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

package controller

import (
	"context"
	"strings"

	istio_network_v1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	apps_v1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
	"github.com/changqings/some-app-operator/pkg/deployment"
	"github.com/changqings/some-app-operator/pkg/hpa"
	"github.com/changqings/some-app-operator/pkg/istio"
	"github.com/changqings/some-app-operator/pkg/service"
)

const (
	CONTAINER_APP_NAME = "app"
	STATUS_RUNNING     = "Running"
	STATUS_UPDATING    = "Updatting"
	STATUS_CREATE      = "Creating"
	STATUS_ERROR       = "Error"
)

// SomeappReconciler reconciles a Someapp object
type SomeappReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	//
	Recorder record.EventRecorder
}

//+kubebuilder:rbac:groups=ops.some.cn,resources=someapps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=ops.some.cn,resources=someapps/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=ops.some.cn,resources=someapps/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=*
//+kubebuilder:rbac:groups=core,resources=services,verbs=*
//+kubebuilder:rbac:groups=autoscaling,resources=horizontalpodautoscalers,verbs=*
//+kubebuilder:rbac:groups=networking.istio.io,resources=virtualservices,verbs=*
//+kubebuilder:rbac:groups=networking.istio.io,resources=destinationrules,verbs=*

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.

// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *SomeappReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx).WithValues("someapp-reconcile", req.NamespacedName)
	// timeAfter := time.Second * 60
	record := r.Recorder

	someApp := &opsv1.Someapp{}
	result := ctrl.Result{}

	err := r.Get(ctx, req.NamespacedName, someApp)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			return result, nil
		}
		record.Eventf(someApp, core_v1.EventTypeWarning, "reconcile err", "msg", err)
		log.Error(err, "r.Get()", "not found someApp")
		return result, client.IgnoreNotFound(err)
	}

	// there are getting someApp, do some operation
	if someApp.Spec.CanaryTag != "stable" {
		someApp.Spec.AppVersion = "canary"
	}

	// deployment reconcile
	sd := deployment.SomeDeployment{}
	err = sd.Reconcile(ctx, someApp, r.Client, r.Scheme, log)
	if err != nil {
		someApp.Status.Status.Phase = STATUS_ERROR
		r.Status().Update(ctx, someApp)
		return result, err
	}

	// hpa
	if len(someApp.Spec.HpaNums) > 0 {
		sh := hpa.SomeHpa{}
		err = sh.Reconcile(ctx, someApp, r.Client, r.Scheme, log)
		if err != nil {
			someApp.Status.Status.Phase = STATUS_ERROR
			r.Status().Update(ctx, someApp)
			return result, err
		}
	}

	// svc
	if strings.HasPrefix(someApp.Spec.AppType, "api") {
		sv := service.SomeService{}
		err = sv.Reconcile(ctx, someApp, r.Client, r.Scheme, log)
		if err != nil {
			someApp.Status.Status.Phase = STATUS_ERROR
			r.Status().Update(ctx, someApp)
			return result, err
		}

	}
	// istio
	if someApp.Spec.EnableIstio {
		si := istio.SomeIstio{}
		err = si.Reconcile(ctx, someApp, r.Client, r.Scheme, log)
		if err != nil {
			someApp.Status.Status.Phase = STATUS_ERROR
			r.Status().Update(ctx, someApp)
			return result, err
		}
	}

	someApp.Status.Status.Phase = STATUS_RUNNING
	r.Status().Update(ctx, someApp)
	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SomeappReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1.Someapp{}).
		Owns(&apps_v1.Deployment{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&core_v1.Service{}).
		Owns(&istio_network_v1beta1.DestinationRule{}).
		Owns(&istio_network_v1beta1.VirtualService{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
