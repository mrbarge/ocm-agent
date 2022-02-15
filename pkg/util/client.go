package util

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func BuildClient() (client.Client, error) {
	cfg := ctrl.GetConfigOrDie()
	scheme := runtime.NewScheme()
	return client.New(cfg, client.Options{
		Scheme: scheme,
	})
}

func BuildDynamicClient() (dynamic.Interface, error) {
	cfg := ctrl.GetConfigOrDie()
	return dynamic.NewForConfig(cfg)
}
