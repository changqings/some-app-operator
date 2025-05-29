package deployment

import (
	"context"
	"strings"

	apps_v1 "k8s.io/api/apps/v1"
	core_v1 "k8s.io/api/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	opsv1 "github.com/changqings/some-app-operator/api/v1"
	"github.com/go-logr/logr"
)

type SomeDeployment struct {
	StandardLabels map[string]string
}

func (sd *SomeDeployment) Reconcile(ctx context.Context, someApp *opsv1.Someapp, client client.Client, scheme *runtime.Scheme, log logr.Logger) error {

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
	)

	// reconcile deployment
	deployment := &apps_v1.Deployment{ObjectMeta: meta_v1.ObjectMeta{
		Name:      sd.StandardLabels["name"],
		Namespace: someApp.Namespace,
	}}

	op, err := controllerutil.CreateOrUpdate(ctx, client, deployment, func() error {

		// check deployment existed
		// spec.selector is immutable, so set it when create
		if deployment.ObjectMeta.CreationTimestamp.IsZero() {
			deployment.ObjectMeta.Labels = sd.StandardLabels
			deployment.Spec.Selector = &meta_v1.LabelSelector{
				MatchLabels: sd.StandardLabels,
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

		for i, c := range someApp.Spec.Containers {
			if c.Name == "app" {
				appContainerIndex = i
				break
			}
		}

		// create or update deployment with template
		deployment.Spec.Template = core_v1.PodTemplateSpec{
			ObjectMeta: meta_v1.ObjectMeta{
				Labels: sd.StandardLabels,
			},
			Spec: core_v1.PodSpec{
				Containers: someApp.Spec.Containers,
			},
		}

		if len(someApp.Spec.ImagePullSecret) > 0 {
			deployment.Spec.Template.Spec.ImagePullSecrets = []core_v1.LocalObjectReference{
				{
					Name: someApp.Spec.ImagePullSecret,
				},
			}
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
					SubPath:   volumeMountFileName,
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
					SubPath:   volumeMountFileName,
				},
			}
		case volumeTypeUnknown:
			log.Info("volume type unknown", "only start with configmap- or secret- will work, someVolume", someVolume)
		}

		// add reference
		if err := controllerutil.SetOwnerReference(someApp, deployment, scheme); err != nil {
			return err
		}

		return nil
	})
	if err != nil {
		return err
	}

	log.Info("deployment reconcile success", "operation_result", op)
	return nil

}
