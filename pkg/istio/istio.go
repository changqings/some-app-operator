package istio

import (
	"context"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	pkgClient "sigs.k8s.io/controller-runtime/pkg/client"
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
	StandardLabels map[string]string
	Stage          string
}

func (si *SomeIstio) Reconcile(ctx context.Context, someApp *opsv1.Someapp, c pkgClient.Client, scheme *runtime.Scheme, log logr.Logger) error {
	var (
		svcHost          = someApp.Spec.AppName + "." + someApp.Namespace + "." + "svc.cluster.local"
		vsHttpRouterName = someApp.Spec.AppName + "-" + someApp.Spec.AppVersion
		drName           = someApp.Spec.AppName
		drExist          = false
		subsetName       = strings.ReplaceAll(someApp.Spec.AppVersion, ".", "-")
	)

	if si.Stage == "canary" {
		svcHost = someApp.Spec.AppName + "-canary." + someApp.Namespace + "." + "svc.cluster.local"
		vsHttpRouterName = someApp.Spec.AppName + "-" + subsetName
		drName = someApp.Spec.AppName + "-canary"
	}

	vs := &istio_network_v1beta1.VirtualService{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Spec.AppName,
		Namespace: someApp.Namespace,
	}}

	dr := &istio_network_v1beta1.DestinationRule{ObjectMeta: meta_v1.ObjectMeta{
		Name:      drName,
		Namespace: someApp.Namespace,
	}}

	// create or patch dr
	// check dr exist,
	dr_key := pkgClient.ObjectKeyFromObject(dr)
	err_get_dr := c.Get(ctx, dr_key, dr)
	if err_get_dr == nil {
		drExist = true
	}

	if si.Stage == opsv1.StableStage || !drExist {
		op_dr, err := controllerutil.CreateOrUpdate(ctx, c, dr, func() error {

			if dr.ObjectMeta.CreationTimestamp.IsZero() {
				dr.ObjectMeta.Labels = si.StandardLabels
			}

			dr.Spec = istio_api_network_v1beta1.DestinationRule{
				Host: svcHost,
				Subsets: []*istio_api_network_v1beta1.Subset{
					{
						Labels: map[string]string{
							"version": someApp.Spec.AppVersion,
						},
						Name: subsetName,
					},
				},
			}

			if err := controllerutil.SetOwnerReference(someApp, dr, scheme); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return err
		}
		log.Info("dr reconcile success", "operation_result", op_dr)

	} else if si.Stage == opsv1.CanaryStage && drExist {

		var subsetExisting bool

		// check subset.Name already exist in dr
		for _, v := range dr.Spec.Subsets {
			if v.Name == subsetName {
				subsetExisting = true
				break
			}
		}

		// do patch
		if !subsetExisting {
			existing_dr := dr.DeepCopy()

			existing_dr.Spec.Subsets = append(existing_dr.Spec.Subsets, &istio_api_network_v1beta1.Subset{
				Labels: map[string]string{
					"version": someApp.Spec.AppVersion,
				},
				Name: subsetName,
			})

			// update
			if err := c.Update(ctx, existing_dr); err != nil {
				return err
			}
			log.Info("canary dr reconcile success", "operation_result", controllerutil.OperationResultUpdated)

		}

	}

	// create or patch vs
	// stage on stable, create stable vs
	// stabe on canary, patch on stable vs
	if si.Stage == opsv1.StableStage {
		op_vs, err := controllerutil.CreateOrUpdate(ctx, c, vs, func() error {

			if vs.ObjectMeta.CreationTimestamp.IsZero() {
				vs.ObjectMeta.Labels = si.StandardLabels
			}

			vs.Spec = istio_api_network_v1beta1.VirtualService{
				Gateways: []string{"mesh"},
				Hosts: []string{
					svcHost,
				},
				Http: []*istio_api_network_v1beta1.HTTPRoute{
					{
						Name: vsHttpRouterName,
						Route: []*istio_api_network_v1beta1.HTTPRouteDestination{
							{
								Destination: &istio_api_network_v1beta1.Destination{
									Host:   svcHost,
									Subset: subsetName,
								},
								Weight: 0,
							},
						},
					},
				},
			}
			//used with careful, should turn off this on production
			if err := controllerutil.SetOwnerReference(someApp, vs, scheme); err != nil {
				return err
			}

			return nil
		})

		if err != nil {
			return err
		}

		log.Info("vs reconcile success", "operation_result", op_vs)
	} else {
		// do patch

		//get
		key := pkgClient.ObjectKeyFromObject(vs)
		if err := c.Get(ctx, key, vs); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		// modify
		var indexStableRouter int

		for i, v := range vs.Spec.Http {
			if v.Name == someApp.Spec.AppName+"-stable" {
				indexStableRouter = i
			}
		}

		existing_vs := vs.DeepCopy()

		for i, v := range existing_vs.Spec.Http[indexStableRouter].Route {
			if v.Destination.Subset == "stable" {
				existing_vs.Spec.Http[indexStableRouter].Route[i].Weight = 100
			}
		}

		canaryHttpRouter := &istio_api_network_v1beta1.HTTPRoute{
			Name: vsHttpRouterName,
			Route: append(existing_vs.Spec.Http[indexStableRouter].Route, &istio_api_network_v1beta1.HTTPRouteDestination{
				Destination: &istio_api_network_v1beta1.Destination{
					Host:   svcHost,
					Subset: subsetName,
				},
				Weight: 0,
			}),
		}

		existing_vs.Spec.Http = append(existing_vs.Spec.Http[:indexStableRouter],
			append([]*istio_api_network_v1beta1.HTTPRoute{canaryHttpRouter}, existing_vs.Spec.Http[indexStableRouter:]...)...)

		// update
		if err := c.Update(ctx, existing_vs); err != nil {
			return err
		}
		log.Info("canary vs reconcile success", "operation_result", controllerutil.OperationResultUpdated)

	}

	return nil

}
