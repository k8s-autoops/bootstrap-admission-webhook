package main

import (
	"context"
	"fmt"
	"github.com/k8s-autoops/autoops"
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

	secretNameAdmissionCert := admissionName + "-cert"
	admissionDNSNames := []string{
		admissionName,
		admissionName + "." + namespace,
		admissionName + "." + namespace + ".svc",
		admissionName + "." + namespace + ".svc.cluster",
		admissionName + "." + namespace + ".svc.cluster.local",
	}

	if certPEM, keyPEM, err = autoops.EnsureSecretAsKeyPair(
		context.Background(),
		client,
		namespace,
		secretNameAdmissionCert,
		autoops.KeyPairOptions{
			CACertPEM: caCertPEM,
			CAKeyPEM:  caKeyPEM,
			DNSNames:  admissionDNSNames,
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
			Name: serviceName,
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
}
