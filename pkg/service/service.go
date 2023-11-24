package service

import (
	"context"

	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8s_utils_pointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
	"github.com/go-logr/logr"
)

// for safe, service not add ownerReference, plase delete it manually
// only select someApp.Spec.AppType="api"
// labelSelector  targetPort="http"
type SomeService struct {
	StandardLabels map[string]string
}

func (sv *SomeService) Reconcile(ctx context.Context, someApp *opsv1.Someapp, client client.Client, scheme *runtime.Scheme, log logr.Logger) error {

	var (
		selectTargetLabels = map[string]string{
			"name":    someApp.Name,
			"type":    someApp.Spec.AppType,
			"app":     someApp.Spec.AppName,
			"version": someApp.Spec.AppVersion,
		}
		appContainerPort  int32
		appContainerIndex int
		someAppContainer  = someApp.Spec.Containers
	)

	for i, c := range someApp.Spec.Containers {
		if c.Name == "app" {
			appContainerIndex = i
			break
		}
	}

	for _, v := range someAppContainer[appContainerIndex].Ports {
		appContainerPort = v.ContainerPort
	}

	// reconcile deployment
	service := &core_v1.Service{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Name,
		Namespace: someApp.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, client, service, func() error {

		if service.ObjectMeta.CreationTimestamp.IsZero() {
			service.ObjectMeta.Labels = sv.StandardLabels
		}

		if service.ResourceVersion != "" {
			service.ResourceVersion = "0"
		}

		service.Spec = core_v1.ServiceSpec{
			Selector: selectTargetLabels,
			Type:     core_v1.ServiceTypeClusterIP,
			Ports: []core_v1.ServicePort{
				{
					Name:        "http",
					Protocol:    "TCP",
					Port:        80,
					TargetPort:  intstr.FromInt32(appContainerPort),
					AppProtocol: k8s_utils_pointer.String("http"),
				},
			},
		}
		if err := controllerutil.SetOwnerReference(someApp, service, scheme); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("service reconcile success", "operation_result", op)
	return nil

}
