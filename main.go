package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/k8s-autoops/autoops"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
	"strconv"
	"strings"
)

const (
	SecretAdmissionBootstrapperCA = "admission-bootstrapper-ca"
)

func exit(err *error) {
	if *err != nil {
		log.Println("exited with error:", (*err).Error())
		os.Exit(1)
	} else {
		log.Println("exited")
	}
}

func main() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ltime | log.Lmsgprefix)

	var err error
	defer exit(&err)

	envAdmissionName := strings.TrimSpace(os.Getenv("ADMISSION_NAME"))
	if envAdmissionName == "" {
		err = fmt.Errorf("missing environment variable: ADMISSION_NAME")
		return
	}
	envAdmissionImage := strings.TrimSpace(os.Getenv("ADMISSION_IMAGE"))
	if envAdmissionImage == "" {
		err = fmt.Errorf("missing environment variable: ADMISSION_IMAGE")
		return
	}
	envAdmissionEnvs := strings.TrimSpace(os.Getenv("ADMISSION_ENVS"))
	envAdmissionMutating, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("ADMISSION_MUTATING")))
	envAdmissionRules := strings.TrimSpace(os.Getenv("ADMISSION_RULES"))
	envAdmissionSideEffect := strings.TrimSpace(os.Getenv("ADMISSION_SIDE_EFFECT"))
	if envAdmissionSideEffect == "" {
		envAdmissionSideEffect = string(admissionregistrationv1.SideEffectClassUnknown)
	}
	envAdmissionIgnoreFailure, _ := strconv.ParseBool(strings.TrimSpace(os.Getenv("ADMISSION_IGNORE_FAILURE")))
	envAdmissionServiceAccount := strings.TrimSpace(os.Getenv("ADMISSION_SERVICE_ACCOUNT"))

	var client *kubernetes.Clientset
	if client, err = autoops.InClusterClient(); err != nil {
		return
	}

	var namespace string
	if namespace, err = autoops.CurrentNamespace(); err != nil {
		return
	}

	var (
		caCertPEM []byte
		caKeyPEM  []byte
	)

	if caCertPEM, caKeyPEM, err = autoops.EnsureSecretAsKeyPair(
		context.Background(),
		client,
		namespace,
		SecretAdmissionBootstrapperCA,
		autoops.KeyPairOptions{},
	); err != nil {
		return
	}

	log.Println("Bootstrapper CA Ensured:\n", string(caCertPEM))

	var certPEM []byte

	secretNameCert := envAdmissionName + "-cert"

	if certPEM, _, err = autoops.EnsureSecretAsKeyPair(
		context.Background(),
		client,
		namespace,
		secretNameCert,
		autoops.KeyPairOptions{
			CACertPEM: caCertPEM,
			CAKeyPEM:  caKeyPEM,
			DNSNames: []string{
				envAdmissionName,
				envAdmissionName + "." + namespace,
				envAdmissionName + "." + namespace + ".svc",
				envAdmissionName + "." + namespace + ".svc.cluster",
				envAdmissionName + "." + namespace + ".svc.cluster.local",
			},
		},
	); err != nil {
		return
	}

	log.Println("Admission Cert Ensured:\n" + string(certPEM))

	serviceSelector := map[string]string{
		"k8s-app": envAdmissionName,
	}

	if _, err = autoops.ServiceGetOrCreate(context.Background(), client, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      envAdmissionName,
		},
		Spec: corev1.ServiceSpec{
			Selector: serviceSelector,
			Type:     corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name:       "https",
					Protocol:   corev1.ProtocolTCP,
					Port:       443,
					TargetPort: intstr.FromInt(443),
				},
			},
		},
	}); err != nil {
		return
	}

	log.Println("Service Ensured:", envAdmissionName)

	var statefulsetEnvVars []corev1.EnvVar
	envSplits := strings.Split(envAdmissionEnvs, ";")
	for _, envSplit := range envSplits {
		kvSplits := strings.Split(envSplit, "=")
		if len(kvSplits) != 2 {
			continue
		}
		k, v := strings.TrimSpace(kvSplits[0]), strings.TrimSpace(kvSplits[1])
		if k == "" {
			continue
		}
		statefulsetEnvVars = append(statefulsetEnvVars, corev1.EnvVar{Name: k, Value: v})
	}

	if _, err = autoops.StatefulSetGetOrCreate(
		context.Background(),
		client,
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      envAdmissionName,
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: serviceSelector,
				},
				ServiceName: envAdmissionName,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: serviceSelector,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            envAdmissionName,
								Image:           envAdmissionImage,
								ImagePullPolicy: corev1.PullAlways,
								Env:             statefulsetEnvVars,
								Ports: []corev1.ContainerPort{
									{
										Name:          "https",
										Protocol:      corev1.ProtocolTCP,
										ContainerPort: 443,
									},
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "vol-tls",
										SubPath:   corev1.TLSCertKey,
										MountPath: autoops.AdmissionServerCertFile,
									},
									{
										Name:      "vol-tls",
										SubPath:   corev1.TLSPrivateKeyKey,
										MountPath: autoops.AdmissionServerKeyFile,
									},
								},
							},
						},
						ServiceAccountName: envAdmissionServiceAccount,
						Volumes: []corev1.Volume{
							{
								Name: "vol-tls",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretNameCert,
									},
								},
							},
						},
					},
				},
			},
		}); err != nil {
		return
	}

	log.Println("Statefulset Ensured:", envAdmissionName)

	var admissionRules []admissionregistrationv1.RuleWithOperations
	if err = json.Unmarshal([]byte(envAdmissionRules), &admissionRules); err != nil {
		return
	}
	admissionSideEffect := admissionregistrationv1.SideEffectClass(envAdmissionSideEffect)
	var admissionFailurePolicy *admissionregistrationv1.FailurePolicyType
	if envAdmissionIgnoreFailure {
		admissionFailurePolicy = new(admissionregistrationv1.FailurePolicyType)
		*admissionFailurePolicy = admissionregistrationv1.Ignore
	}

	if envAdmissionMutating {
		if _, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Get(
			context.Background(),
			envAdmissionName,
			metav1.GetOptions{},
		); err != nil {
			if errors.IsNotFound(err) {
				err = nil
			} else {
				return
			}
		} else {
			goto admissionEnsured
		}

		if _, err = client.AdmissionregistrationV1().MutatingWebhookConfigurations().Create(
			context.Background(),
			&admissionregistrationv1.MutatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: envAdmissionName,
				},
				Webhooks: []admissionregistrationv1.MutatingWebhook{
					{
						Name: envAdmissionName + ".k8s-autoops.github.io",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: caCertPEM,
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: namespace,
								Name:      envAdmissionName,
							},
						},
						Rules:                   admissionRules,
						SideEffects:             &admissionSideEffect,
						FailurePolicy:           admissionFailurePolicy,
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			},
			metav1.CreateOptions{},
		); err != nil {
			return
		}
	} else {
		if _, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Get(
			context.Background(),
			envAdmissionName,
			metav1.GetOptions{},
		); err != nil {
			if errors.IsNotFound(err) {
				err = nil
			} else {
				return
			}
		} else {
			goto admissionEnsured
		}

		if _, err = client.AdmissionregistrationV1().ValidatingWebhookConfigurations().Create(
			context.Background(),
			&admissionregistrationv1.ValidatingWebhookConfiguration{
				ObjectMeta: metav1.ObjectMeta{
					Name: envAdmissionName,
				},
				Webhooks: []admissionregistrationv1.ValidatingWebhook{
					{
						Name: envAdmissionName + ".k8s-autoops.github.io",
						ClientConfig: admissionregistrationv1.WebhookClientConfig{
							CABundle: caCertPEM,
							Service: &admissionregistrationv1.ServiceReference{
								Namespace: namespace,
								Name:      envAdmissionName,
							},
						},
						Rules:                   admissionRules,
						SideEffects:             &admissionSideEffect,
						FailurePolicy:           admissionFailurePolicy,
						AdmissionReviewVersions: []string{"v1"},
					},
				},
			},
			metav1.CreateOptions{},
		); err != nil {
			return
		}
	}

admissionEnsured:
	log.Println("AdmissionWebHook ensured:", envAdmissionName)
}
