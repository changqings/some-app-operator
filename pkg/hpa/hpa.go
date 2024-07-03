package hpa

import (
	"context"
	"strconv"
	"strings"

	autoscalingv2 "k8s.io/api/autoscaling/v2"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8s_utils_pointer "k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
	"github.com/go-logr/logr"
)

type SomeHpa struct {
	StandardLabels map[string]string
}

func (sh *SomeHpa) Reconcile(ctx context.Context, someApp *opsv1.Someapp, client client.Client, scheme *runtime.Scheme, log logr.Logger) error {

	var (
		someHpaNums    = someApp.Spec.SetHpa
		hpaMin, hpaMax int64
	)

	// reconcile hpa
	hpa := &autoscalingv2.HorizontalPodAutoscaler{
		ObjectMeta: meta_v1.ObjectMeta{
			Name:      sh.StandardLabels["name"],
			Namespace: someApp.Namespace,
		},
	}

	op, err := controllerutil.CreateOrUpdate(ctx, client, hpa, func() error {

		// check  if existed, if not do something
		if hpa.ObjectMeta.CreationTimestamp.IsZero() {
			hpa.ObjectMeta.Labels = sh.StandardLabels
		}

		hpaMinMaxs := strings.Split(someHpaNums, "->")
		if len(hpaMinMaxs) == 2 {
			hpaMin, _ = strconv.ParseInt(hpaMinMaxs[0], 10, 64)
			hpaMax, _ = strconv.ParseInt(hpaMinMaxs[1], 10, 64)
			if hpaMin > hpaMax {
				tmpNum := hpaMax
				hpaMax = hpaMin
				hpaMin = tmpNum
			}
		}

		hpa.Spec = autoscalingv2.HorizontalPodAutoscalerSpec{
			MinReplicas: k8s_utils_pointer.Int32(int32(hpaMin)),
			MaxReplicas: int32(hpaMax),
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       sh.StandardLabels["name"],
			},
			Metrics: []autoscalingv2.MetricSpec{
				{
					Type: autoscalingv2.ResourceMetricSourceType,
					Resource: &autoscalingv2.ResourceMetricSource{
						Name: core_v1.ResourceCPU,
						Target: autoscalingv2.MetricTarget{
							AverageUtilization: &someApp.Spec.HpaCpuUsage,
							Type:               autoscalingv2.UtilizationMetricType,
						},
					},
				},
			},
		}

		// add reference
		if err := controllerutil.SetOwnerReference(someApp, hpa, scheme); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return err
	}

	log.Info("hpa reconcile success", "operation_result", op)
	return nil

}
