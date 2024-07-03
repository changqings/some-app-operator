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
	Stage            string
	DeleteAction     bool
	svcHost          string
	vsHttpRouterName string
	drName           string
	subsetName       string
}

func (si *SomeIstio) Reconcile(ctx context.Context, someApp *opsv1.Someapp, c pkgClient.Client, scheme *runtime.Scheme, log logr.Logger) error {

	si.svcHost = someApp.Spec.AppName + "." + someApp.Namespace + "." + "svc.cluster.local"
	si.vsHttpRouterName = someApp.Spec.AppName + "-" + someApp.Spec.AppVersion
	si.drName = someApp.Spec.AppName
	si.subsetName = strings.ReplaceAll(someApp.Spec.AppVersion, ".", "-")

	if si.Stage == opsv1.CanaryStage {
		si.svcHost = someApp.Spec.AppName + "-canary." + someApp.Namespace + "." + "svc.cluster.local"
		si.vsHttpRouterName = someApp.Spec.AppName + "-" + si.subsetName
		si.drName = someApp.Spec.AppName + "-canary"
	}

	if err := si.reconcileVs(ctx, someApp, c, scheme, log); err != nil {
		return err
	}

	if err := si.reconcileDr(ctx, someApp, c, scheme, log); err != nil {
		return err
	}

	return nil
}

func (si *SomeIstio) reconcileVs(ctx context.Context, someApp *opsv1.Someapp, c pkgClient.Client, scheme *runtime.Scheme, log logr.Logger) error {

	vs := &istio_network_v1beta1.VirtualService{ObjectMeta: meta_v1.ObjectMeta{
		Name:      someApp.Spec.AppName,
		Namespace: someApp.Namespace,
	}}

	// stable stage, create vs
	if si.Stage == opsv1.StableStage && !si.DeleteAction {
		op_vs, err := controllerutil.CreateOrUpdate(ctx, c, vs, func() error {

			if vs.ObjectMeta.CreationTimestamp.IsZero() {
				vs.ObjectMeta.Labels = map[string]string{
					"app":   someApp.Spec.AppName,
					"type":  someApp.Spec.AppType,
					"stage": si.Stage,
				}
			}

			vs.Spec = istio_api_network_v1beta1.VirtualService{
				Gateways: []string{"mesh"},
				Hosts: []string{
					si.svcHost,
				},
				Http: []*istio_api_network_v1beta1.HTTPRoute{
					{
						Name: si.vsHttpRouterName,
						Route: []*istio_api_network_v1beta1.HTTPRouteDestination{
							{
								Destination: &istio_api_network_v1beta1.Destination{
									Host:   si.svcHost,
									Subset: si.subsetName,
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
		return nil
	}

	// canary stage, delete or patch vs
	key := pkgClient.ObjectKeyFromObject(vs)
	if err := c.Get(ctx, key, vs); err != nil {
		if apierrors.IsNotFound(err) {
			log.Error(err, "stable vs not found", "vs_name", vs.Name, "vs_namespace", vs.Namespace)
			return nil
		}
		return err
	}
	existing_vs := vs.DeepCopy()

	// modify
	var stableRouterIndex int
	var canaryRouterExist bool
	var canaryRouterIndex int

	for i, v := range existing_vs.Spec.Http {
		if v.Name == someApp.Spec.AppName+"-stable" {
			stableRouterIndex = i
			break
		}
	}

	for i, v := range existing_vs.Spec.Http {
		if v.Name == si.vsHttpRouterName {
			canaryRouterExist = true
			canaryRouterIndex = i
			break
		}
	}

	// do delete
	if si.DeleteAction && canaryRouterExist {

		existing_vs.Spec.Http = append(existing_vs.Spec.Http[:canaryRouterIndex], existing_vs.Spec.Http[canaryRouterIndex+1:]...)

	} else if !si.DeleteAction && !canaryRouterExist {

		for i, v := range existing_vs.Spec.Http[stableRouterIndex].Route {
			if v.Destination.Subset == "stable" {
				existing_vs.Spec.Http[stableRouterIndex].Route[i].Weight = 100
			}
		}

		canaryHttpRouter := &istio_api_network_v1beta1.HTTPRoute{
			Name: si.vsHttpRouterName,
			Route: append(existing_vs.Spec.Http[stableRouterIndex].Route, &istio_api_network_v1beta1.HTTPRouteDestination{
				Destination: &istio_api_network_v1beta1.Destination{
					Host:   si.svcHost,
					Subset: si.subsetName,
				},
				Weight: 0,
			}),
		}

		existing_vs.Spec.Http = append(existing_vs.Spec.Http[:stableRouterIndex],
			append([]*istio_api_network_v1beta1.HTTPRoute{canaryHttpRouter}, existing_vs.Spec.Http[stableRouterIndex:]...)...)
	}

	// update
	if err := c.Update(ctx, existing_vs); err != nil {
		return err
	}
	log.Info("canary vs reconcile success", "operation_result", controllerutil.OperationResultUpdated)

	return nil
}
func (si *SomeIstio) reconcileDr(ctx context.Context, someApp *opsv1.Someapp, c pkgClient.Client, scheme *runtime.Scheme, log logr.Logger) error {

	dr := &istio_network_v1beta1.DestinationRule{ObjectMeta: meta_v1.ObjectMeta{
		Name:      si.drName,
		Namespace: someApp.Namespace,
	}}

	// create or patch dr
	// check dr exist,
	drExist := false
	dr_key := pkgClient.ObjectKeyFromObject(dr)

	if err := c.Get(ctx, dr_key, dr); err == nil {
		drExist = true
	}

	// create dr
	if !si.DeleteAction && (si.Stage == opsv1.StableStage || !drExist) {
		op_dr, err := controllerutil.CreateOrUpdate(ctx, c, dr, func() error {

			// patch canary subsetName on annotation, for delete  use
			if dr.ObjectMeta.CreationTimestamp.IsZero() {
				dr.ObjectMeta.Labels = map[string]string{
					"app":   someApp.Spec.AppName,
					"type":  someApp.Spec.AppType,
					"stage": si.Stage,
				}
			}

			dr.Spec = istio_api_network_v1beta1.DestinationRule{
				Host: si.svcHost,
				Subsets: []*istio_api_network_v1beta1.Subset{
					{
						Labels: map[string]string{
							"version": someApp.Spec.AppVersion,
						},
						Name: si.subsetName,
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
		return nil

	}

	// patch dr
	if si.Stage == opsv1.CanaryStage && drExist {

		subsetExisting := false
		subsetIndex := 0
		existing_dr := dr.DeepCopy()

		// check subset.Name already exist in dr
		for i, v := range existing_dr.Spec.Subsets {
			if v.Name == si.subsetName {
				subsetExisting = true
				subsetIndex = i
				break
			}
		}

		if si.DeleteAction && subsetExisting {
			existing_dr.Spec.Subsets = append(existing_dr.Spec.Subsets[:subsetIndex], existing_dr.Spec.Subsets[subsetIndex+1:]...)
		} else if !si.DeleteAction && !subsetExisting {
			existing_dr.Spec.Subsets = append(existing_dr.Spec.Subsets, &istio_api_network_v1beta1.Subset{
				Labels: map[string]string{
					"version": someApp.Spec.AppVersion,
				},
				Name: si.subsetName,
			})

			if err := controllerutil.SetOwnerReference(someApp, existing_dr, scheme); err != nil {
				return err
			}

		}
		// update
		if err := c.Update(ctx, existing_dr); err != nil {
			return err
		}

		log.Info("canary dr reconcile success", "operation_result", controllerutil.OperationResultUpdated)
	}

	return nil

}
