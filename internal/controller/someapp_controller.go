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
	"time"

	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
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
	// Recorder recorder.Provider
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
	log := log.FromContext(ctx).WithValues("someapp-collector", req.NamespacedName)
	timeAfter := time.Second * 3
	// record := r.Recorder.GetEventRecorderFor("some_app")

	someApp := &opsv1.Someapp{}
	result := ctrl.Result{}

	err := r.Get(ctx, req.NamespacedName, someApp)
	if err != nil {
		if k8s_errors.IsNotFound(err) {
			result.RequeueAfter = timeAfter
			return result, nil
		}
		// record.Event(someApp, core_v1.EventTypeWarning, "reconcile err", "r.Get() err")
		log.Error(err, "r.Get()", "not found someApp")
		return result, client.IgnoreNotFound(err)
	}

	// reconcile deployment
	deployment := &apps_v1.Deployment{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Spec.AppName,
		Namespace: someApp.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, r.Client, deployment, func() error {

		var (
			volumeType          string
			volumeName          string
			appContainerIndex   int
			someVolume          = someApp.Spec.SomeVolume
			volumeTypeConfigMap = "configmap"
			volumeTypeSecret    = "secret"
			volumeTypeUnknown   = "unknown"
			// this is the file name of configMap
			volumeMountFileName = "some_config.yaml"
			standardLabels      = map[string]string{
				"app":     someApp.Spec.AppName,
				"name":    someApp.Name,
				"type":    someApp.Spec.AppType,
				"version": someApp.Spec.AppVersion,
			}
		)

		// check deployment if existed, if not do something
		// spec.selector is immutable, so set it when create
		if deployment.ObjectMeta.CreationTimestamp.IsZero() {
			deployment.ObjectMeta.Labels = standardLabels
			deployment.Spec.Selector = &meta_v1.LabelSelector{
				MatchLabels: standardLabels,
			}

		}

		if n := strings.TrimPrefix(someVolume, "configmap-"); len(n) > 0 {
			volumeType = volumeTypeConfigMap
			volumeName = n
		} else if n := strings.TrimPrefix(someVolume, "secret-"); len(n) > 0 {
			volumeType = volumeTypeSecret
			volumeName = n
		} else {
			volumeType = volumeTypeUnknown
		}

		for i, c := range deployment.Spec.Template.Spec.Containers {
			if c.Name == "app" {
				appContainerIndex = i
				break
			}
		}

		// create or update deployment with default template
		deployment.Spec.Template = core_v1.PodTemplateSpec{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: standardLabels,
			},
			Spec: core_v1.PodSpec{
				Containers: someApp.Spec.Containers,
				ImagePullSecrets: []core_v1.LocalObjectReference{
					{
						Name: someApp.Spec.ImagePullSecret,
					},
				},
			},
		}

		switch volumeType {
		case volumeTypeConfigMap:
			deployment.Spec.Template.Spec.Volumes = []core_v1.Volume{
				{
					Name: volumeName,
					VolumeSource: core_v1.VolumeSource{
						ConfigMap: &core_v1.ConfigMapVolumeSource{
							LocalObjectReference: core_v1.LocalObjectReference{
								Name: volumeName,
							},
						},
					},
				},
			}
			deployment.Spec.Template.Spec.Containers[appContainerIndex].VolumeMounts = []core_v1.VolumeMount{
				{
					Name:      volumeName,
					ReadOnly:  true,
					MountPath: "/app/" + volumeMountFileName,
				},
			}
		case volumeTypeSecret:
			deployment.Spec.Template.Spec.Volumes = []core_v1.Volume{
				{
					Name: volumeName,
					VolumeSource: core_v1.VolumeSource{
						Secret: &core_v1.SecretVolumeSource{
							SecretName: volumeName,
						},
					},
				},
			}
			deployment.Spec.Template.Spec.Containers[appContainerIndex].VolumeMounts = []core_v1.VolumeMount{
				{
					Name:      volumeName,
					ReadOnly:  true,
					MountPath: "/app/" + volumeMountFileName,
				},
			}
		case volumeTypeUnknown:
			log.Info("volume type unknown", "spec.some_volume", someVolume)
		}

		return nil

	})
	if err != nil {
		return result, err
	}
	log.Info("deployment reconcile success", "operation", op)

	// and add another reconcile

	return result, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *SomeappReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&opsv1.Someapp{}).
		Owns(&apps_v1.Deployment{}).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: 1,
		}).
		Complete(r)
}
