package istio

import (
	"context"

	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
	"github.com/go-logr/logr"

	istio_api_network_v1beta1 "istio.io/api/networking/v1beta1"
	istio_network_v1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
)

// for safe, service not add ownerReference, plase delete it manually
// only select someApp.Spec.AppType="api"
// labelSelector  targetPort="http"
type SomeIstio struct {
}

func (si *SomeIstio) Reconcile(ctx context.Context, someApp *opsv1.Someapp, client client.Client, scheme *runtime.Scheme, log logr.Logger) error {
	var (
		svcHost        = someApp.Name + "." + someApp.Namespace + "." + "svc.cluster.local"
		routerName     = someApp.Name + "-stable"
		standardLabels = map[string]string{
			"name":    someApp.Name,
			"app":     someApp.Spec.AppName,
			"type":    someApp.Spec.AppType,
			"version": someApp.Spec.AppVersion,
		}
	)

	vs := &istio_network_v1beta1.VirtualService{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Name,
		Namespace: someApp.Namespace,
	}}

	dr := &istio_network_v1beta1.DestinationRule{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Name,
		Namespace: someApp.Namespace,
	}}

	op_dr, err := controllerutil.CreateOrUpdate(ctx, client, dr, func() error {

		if dr.ObjectMeta.CreationTimestamp.IsZero() {
			dr.ObjectMeta.Labels = standardLabels
		}

		dr.Spec = istio_api_network_v1beta1.DestinationRule{
			Host: svcHost,
			Subsets: []*istio_api_network_v1beta1.Subset{
				{
					Labels: map[string]string{
						"version": "stable",
					},
					Name: "stable",
				},
			},
		}

		return nil
	})

	if err != nil {
		return err
	}
	log.Info("dr reconcile success", "operation_result", op_dr)

	op_vs, err := controllerutil.CreateOrUpdate(ctx, client, vs, func() error {

		if vs.ObjectMeta.CreationTimestamp.IsZero() {
			vs.ObjectMeta.Labels = standardLabels
		}

		if vs.ResourceVersion != "" {
			vs.ResourceVersion = "0"
		}

		vs.Spec = istio_api_network_v1beta1.VirtualService{
			Gateways: []string{"mesh"},
			Hosts: []string{
				svcHost,
			},
			Http: []*istio_api_network_v1beta1.HTTPRoute{
				{
					Name: routerName,
					Route: []*istio_api_network_v1beta1.HTTPRouteDestination{
						{
							Destination: &istio_api_network_v1beta1.Destination{
								Host:   svcHost,
								Subset: "stable",
							},
						},
					},
				},
			},
		}

		return nil
	})

	if err != nil {
		return err
	}
	log.Info("vs reconcile success", "operation_result", op_vs)

	return nil

}
