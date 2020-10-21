package main

import (
	"context"
	"fmt"
	"github.com/k8s-autoops/autoops"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"log"
	"os"
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

	admissionName := strings.TrimSpace(os.Getenv("ADMISSION_NAME"))
	if admissionName == "" {
		err = fmt.Errorf("missing environment variable: ADMISSION_NAME")
		return
	}
	admissionImage := strings.TrimSpace(os.Getenv("ADMISSION_IMAGE"))
	if admissionImage == "" {
		err = fmt.Errorf("missing environment variable: ADMISSION_IMAGE")
		return
	}
	admissionCfg := strings.TrimSpace(os.Getenv("ADMISSION_CFG"))
	if admissionCfg == "" {
		err = fmt.Errorf("missing environment variable: ADMISSION_CFG")
		return
	}

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

	var (
		certPEM []byte
		keyPEM  []byte
	)

	secretNameCert := admissionName + "-cert"

	if certPEM, keyPEM, err = autoops.EnsureSecretAsKeyPair(
		context.Background(),
		client,
		namespace,
		secretNameCert,
		autoops.KeyPairOptions{
			CACertPEM: caCertPEM,
			CAKeyPEM:  caKeyPEM,
			DNSNames: []string{
				admissionName,
				admissionName + "." + namespace,
				admissionName + "." + namespace + ".svc",
				admissionName + "." + namespace + ".svc.cluster",
				admissionName + "." + namespace + ".svc.cluster.local",
			},
		},
	); err != nil {
		return
	}

	log.Println("Admission Cert Ensured:\n" + string(certPEM))
	_ = keyPEM

	serviceName := admissionName
	serviceSelector := map[string]string{
		"k8s-app": admissionName,
	}

	if _, err = autoops.ServiceGetOrCreate(context.Background(), client, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      serviceName,
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

	log.Println("Service Ensured:", serviceName)

	statefulsetName := admissionName

	if _, err = autoops.StatefulSetGetOrCreate(
		context.Background(),
		client,
		&appsv1.StatefulSet{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      statefulsetName,
			},
			Spec: appsv1.StatefulSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: serviceSelector,
				},
				ServiceName: serviceName,
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: serviceSelector,
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            statefulsetName,
								Image:           admissionImage,
								ImagePullPolicy: corev1.PullAlways,
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
										MountPath: "/autoops-data/tls",
									},
								},
							},
						},
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

	log.Println("Statefulset Ensured:", serviceName)
}
