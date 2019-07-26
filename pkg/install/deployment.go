/*
Copyright 2018, 2019 the Velero contributors.

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

package install

import (
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type podTemplateOption func(*podTemplateConfig)

type podTemplateConfig struct {
	image                    string
	withoutCredentialsVolume bool
	envVars                  []corev1.EnvVar
	restoreOnly              bool
	annotations              map[string]string
	resources                corev1.ResourceRequirements
	withSecretData           bool
}

func WithImage(image string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.image = image
	}
}

func WithAnnotations(annotations map[string]string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.annotations = annotations
	}
}

func WithoutCredentialsVolume() podTemplateOption {
	return func(c *podTemplateConfig) {
		c.withoutCredentialsVolume = true
	}
}

func WithEnvFromSecretKey(varName, secret, key string) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.envVars = append(c.envVars, corev1.EnvVar{
			Name: varName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: secret,
					},
					Key: key,
				},
			},
		})
	}
}

func WithSecretData(secretPresent bool) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.withSecretData = secretPresent
		// Since there's no secret, don't mount the credentials volume in the pods
		c.withoutCredentialsVolume = true
	}
}

func WithRestoreOnly() podTemplateOption {
	return func(c *podTemplateConfig) {
		c.restoreOnly = true
	}
}

func WithResources(resources corev1.ResourceRequirements) podTemplateOption {
	return func(c *podTemplateConfig) {
		c.resources = resources
	}
}

func Deployment(namespace string, opts ...podTemplateOption) *appsv1.Deployment {
	// TODO: Add support for server args
	c := &podTemplateConfig{
		image: DefaultImage,
	}

	for _, opt := range opts {
		opt(c)
	}

	pullPolicy := corev1.PullAlways
	imageParts := strings.Split(c.image, ":")
	if len(imageParts) == 2 && imageParts[1] != "latest" {
		pullPolicy = corev1.PullIfNotPresent

	}

	containerLabels := labels()
	containerLabels["deploy"] = "velero"

	deployment := &appsv1.Deployment{
		ObjectMeta: objectMeta(namespace, "velero"),
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: appsv1.SchemeGroupVersion.String(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"deploy": "velero"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      containerLabels,
					Annotations: podAnnotations(c.annotations),
				},
				Spec: corev1.PodSpec{
					RestartPolicy:      corev1.RestartPolicyAlways,
					ServiceAccountName: "velero",
					Containers: []corev1.Container{
						{
							Name:            "velero",
							Image:           c.image,
							Ports:           containerPorts(),
							ImagePullPolicy: pullPolicy,
							Command: []string{
								"/velero",
							},
							Args: []string{
								"server",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "plugins",
									MountPath: "/plugins",
								},
								{
									Name:      "scratch",
									MountPath: "/scratch",
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "VELERO_SCRATCH_DIR",
									Value: "/scratch",
								},
							},
							Resources: c.resources,
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "plugins",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "scratch",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: new(corev1.EmptyDirVolumeSource),
							},
						},
					},
				},
			},
		},
	}

	if !c.withoutCredentialsVolume {
		deployment.Spec.Template.Spec.Volumes = append(
			deployment.Spec.Template.Spec.Volumes,
			corev1.Volume{
				Name: "cloud-credentials",
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: "cloud-credentials",
					},
				},
			},
		)

		deployment.Spec.Template.Spec.Containers[0].VolumeMounts = append(
			deployment.Spec.Template.Spec.Containers[0].VolumeMounts,
			corev1.VolumeMount{
				Name:      "cloud-credentials",
				MountPath: "/credentials",
			},
		)
	}

	if c.withSecretData {
		deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, []corev1.EnvVar{
			{
				Name:  "GOOGLE_APPLICATION_CREDENTIALS",
				Value: "/credentials/cloud",
			},
			{
				Name:  "AWS_SHARED_CREDENTIALS_FILE",
				Value: "/credentials/cloud",
			},
			{
				Name:  "AZURE_CREDENTIALS_FILE",
				Value: "/credentials/cloud",
			},
		}...)
	}

	deployment.Spec.Template.Spec.Containers[0].Env = append(deployment.Spec.Template.Spec.Containers[0].Env, c.envVars...)

	if c.restoreOnly {
		deployment.Spec.Template.Spec.Containers[0].Args = append(deployment.Spec.Template.Spec.Containers[0].Args, "--restore-only")
	}

	return deployment
}
