package main

import (
	"encoding/json"
	"fmt"
	"log"

	k "github.com/mcluseau/kubeclient"
	corev1 "k8s.io/api/core/v1"
	ext "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/mcluseau/kgate/config"
)

var (
	serviceName  string
	servicePort  int
	localPort    int
	remoteTarget string
)

func exposeRemoteCommand() *Command {
	cmd := &Command{
		Use: "expose-remote",
		Run: exposeRemoteRun,
	}

	flags := cmd.Flags()
	flags.IntVar(&localPort, "local-port", 0, "Local server port to forward")
	flags.StringVar(&serviceName, "service", "", "Local service name")
	flags.IntVar(&servicePort, "service-port", 0, "Local service port")
	flags.StringVar(&remoteTarget, "remote-target", "", "Remote target to forward to")

	return cmd
}

func exposeRemoteRun(cmd *Command, args []string) {
	if serviceName == "" {
		log.Fatal("Service name is required")
	}

	if localPort == 0 {
		log.Fatal("Local port is required")
	}

	if remoteTarget == "" {
		log.Fatal("Remote target is required")
	}

	if servicePort == 0 {
		servicePort = localPort
	}

	dep, cfg := fetchConfig()
	if cfg.LocalTransfers == nil {
		cfg.LocalTransfers = map[int]*config.TransferTarget{}
	}
	cfg.LocalTransfers[localPort] = &config.TransferTarget{remoteTarget}
	setConfig(dep, cfg)

	// update the service
	svc := getOrCreateService()

	portFound := false
	for _, port := range svc.Spec.Ports {
		if port.Port == int32(servicePort) {
			port.TargetPort = intstr.FromInt(localPort)
			portFound = true
			break
		}
	}

	if !portFound {
		svc.Spec.Ports = append(svc.Spec.Ports, portSpec())
	}

	services := k.Client().CoreV1().Services(namespace)
	if _, err := services.Update(svc); err != nil {
		log.Fatal(err)
	}
}

func portSpec() corev1.ServicePort {
	return corev1.ServicePort{
		Name:       fmt.Sprintf("p%d", servicePort),
		Port:       int32(servicePort),
		TargetPort: intstr.FromInt(localPort),
	}
}

func fetchConfig() (*ext.Deployment, *config.Config) {
	dep, err := k.Client().ExtensionsV1beta1().Deployments(namespace).Get(serverName, getOpts)
	if err != nil {
		log.Fatal(err)
	}

	cfg := &config.Config{}
	for _, env := range dep.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "CONFIG" {
			if err := json.Unmarshal([]byte(env.Value), cfg); err != nil {
				log.Fatal("failed to parse CONFIG env: ", err)
			}
		}
	}

	return dep, cfg
}

func setConfig(dep *ext.Deployment, cfg *config.Config) {
	ba, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}

	cnt := dep.Spec.Template.Spec.Containers[0]
	for idx, env := range cnt.Env {
		if env.Name == "CONFIG" {
			cnt.Env[idx].Value = string(ba)
			break
		}
	}

	if _, err := k.Client().ExtensionsV1beta1().Deployments(namespace).Update(dep); err != nil {
		log.Fatal(err)
	}
}

func getOrCreateService() *corev1.Service {
	services := k.Client().CoreV1().Services(namespace)
	svc, err := services.Get(serviceName, getOpts)
	if errors.IsNotFound(err) {
		svc = &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceName,
				Namespace: namespace,
			},
			Spec: corev1.ServiceSpec{
				Selector: map[string]string{
					"app": serverName,
				},
				Ports: []corev1.ServicePort{portSpec()},
			},
		}

		svc, err := services.Create(svc)
		if err != nil {
			log.Fatal(err)
		}

		return svc

	} else if err != nil {
		log.Fatal(err)
	}

	return svc
}
