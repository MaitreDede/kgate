package main

import (
	"log"

	corev1 "k8s.io/api/core/v1"
	ext "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	k "github.com/mcluseau/kubeclient"
)

const (
	secretCA     = "kgate-ca"
	secretServer = "kgate-server"
	deployServer = "kgate"
)

var (
	serverName  string
	deployImage string
)

func initCommand() *Command {
	cmd := &Command{
		Use: "init",
		Run: initRun,
	}

	flags := cmd.Flags()
	flags.StringVar(&serverName, "server-name", "kgate", "The server name, for the certificate")
	flags.StringVar(&deployImage, "image", "mcluseau/kgate", "The server's image")

	return cmd
}

func initRun(cmd *Command, args []string) {
	secCA := getOrCreateTLS(secretCA, func() ([]byte, []byte) {
		key, keyPEM := PrivateKeyPEM()
		crtPEM := SelfSignedCertificatePEM("CA", "CA", 5, key)
		return keyPEM, crtPEM
	})

	getOrCreateTLS(secretServer, func() ([]byte, []byte) {
		key, keyPEM := PrivateKeyPEM()
		crtPEM := HostCertificatePEM(secCA.Data, 1, key, serverName)
		return keyPEM, crtPEM
	})

	deploys := k.Client().ExtensionsV1beta1().Deployments(namespace)
	if _, err := deploys.Get(deployServer, getOpts); errors.IsNotFound(err) {
		log.Print("Creating deployment ", deployServer)

		var one int32 = 1

		dep := &ext.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployServer,
				Namespace: namespace,
			},
			Spec: ext.DeploymentSpec{
				Replicas: &one,
				Selector: selector(),
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": deployServer,
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "kgate",
								Image: deployImage,
								Env: []corev1.EnvVar{
									{
										Name:  "CONFIG",
										Value: "{}",
									},
								},
								Args: []string{
									"server",
									"--http=:80",
									"--ca=/secrets/ca/ca.crt",
									"--crt=/secrets/server/tls.crt",
									"--key=/secrets/server/tls.key",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "ca",
										MountPath: "/secrets/ca",
									},
									{
										Name:      "server",
										MountPath: "/secrets/server",
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "ca",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretCA,
										Items: []corev1.KeyToPath{
											{
												Key:  "tls.crt",
												Path: "ca.crt",
											},
										},
									},
								},
							},
							{
								Name: "server",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: secretServer,
									},
								},
							},
						},
					},
				},
			},
		}
		if _, err := deploys.Create(dep); err != nil {
			log.Fatal(err)
		}

	} else if err != nil {
		log.Fatal(err)
	}

	services := k.Client().CoreV1().Services(namespace)
	if _, err := services.Get(deployServer, getOpts); errors.IsNotFound(err) {
		log.Print("Creating service ", deployServer)

		srv := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      deployServer,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": deployServer,
				},
				Ports: []corev1.ServicePort{
					{
						Name: "http",
						Port: 80,
					},
				},
			},
		}

		if _, err := services.Create(srv); err != nil {
			log.Fatal(err)
		}

	} else if err != nil {
		log.Fatal(err)
	}
}

func selector() *metav1.LabelSelector {
	sel, err := metav1.ParseToLabelSelector("app=" + deployServer)
	if err != nil {
		panic(err)
	}
	return sel
}

func getOrCreateTLS(name string, createKeyCert func() ([]byte, []byte)) *corev1.Secret {
	secrets := k.Client().Core().Secrets(namespace)
	sec, err := secrets.Get(name, getOpts)

	if errors.IsNotFound(err) {
		log.Print("Generating TLS secret ", name)

		key, crt := createKeyCert()

		sec := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: namespace,
			},
			Type: corev1.SecretTypeTLS,
			Data: map[string][]byte{
				"tls.crt": crt,
				"tls.key": key,
			},
		}

		sec, err := secrets.Create(sec)

		if err != nil {
			log.Fatal("failed to create secret ", name, ": ", err)
		}

		return sec

	} else if err != nil {
		log.Fatal("failed to fetch secret ", name, ": ", err)
	}

	return sec
}
