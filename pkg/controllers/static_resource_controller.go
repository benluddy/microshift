/*
Copyright © 2022 MicroShift Contributors

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
package controllers

import (
	"context"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/staticresourcecontroller"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	embedded "github.com/openshift/microshift/assets"
	"github.com/openshift/microshift/pkg/config"
)

type StaticResourceController struct {
	ctrl         *staticresourcecontroller.StaticResourceController
	fakeInformer cache.SharedIndexInformer
}

func NewStaticResourceController(cfg *config.MicroshiftConfig) *StaticResourceController {
	restConfig, err := clientcmd.BuildConfigFromFlags("", cfg.KubeConfigPath(config.KubeAdmin)) // todo
	if err != nil {
		panic(err) // todo
	}

	kubeClient, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		panic(err) // todo
	}

	apiextensionsClient, err := apiextensions.NewForConfig(restConfig)
	if err != nil {
		panic(err) // todo
	}

	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		panic(err) // todo
	}

	kubeInformersForNamespaces := v1helpers.NewKubeInformersForNamespaces(
		kubeClient,
		"",
	)

	fakeInformer := cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return &unstructured.UnstructuredList{}, nil
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return watch.NewFake(), nil
			},
		},
		&unstructured.Unstructured{},
		0,
		cache.Indexers{},
	)

	return &StaticResourceController{
		ctrl: staticresourcecontroller.NewStaticResourceController(
			"microshift-static-resources",
			embedded.Asset,
			[]string{
				"core/namespace-openshift-infra.yaml", // todo
			},
			(&resourceapply.ClientHolder{}).
				WithKubernetes(kubeClient).
				WithAPIExtensionsClient(apiextensionsClient).
				WithDynamicClient(dynamicClient),
			noopOperatorClient{
				informer: fakeInformer,
			},
			events.NewLoggingEventRecorder("static-resource-controller"),
		).AddKubeInformers(kubeInformersForNamespaces),
		fakeInformer: fakeInformer,
	}
}

func (s *StaticResourceController) Name() string {
	return "static-resource-controller"
}

func (s *StaticResourceController) Dependencies() []string {
	return []string{
		"kube-apiserver",
	}
}

func (s *StaticResourceController) Run(ctx context.Context, ready chan<- struct{}, stopped chan<- struct{}) error {
	defer close(stopped)
	go s.fakeInformer.Run(ctx.Done())
	close(ready)
	s.ctrl.Run(ctx, 1)
	return nil
}

// The operator API is not served on MicroShift.
type noopOperatorClient struct {
	informer cache.SharedIndexInformer
}

func (oc noopOperatorClient) Informer() cache.SharedIndexInformer {
	return oc.informer
}

func (noopOperatorClient) GetObjectMeta() (meta *metav1.ObjectMeta, err error) {
	return nil, nil
}

func (noopOperatorClient) GetOperatorState() (spec *operatorv1.OperatorSpec, status *operatorv1.OperatorStatus, resourceVersion string, err error) {
	return &operatorv1.OperatorSpec{ManagementState: operatorv1.Managed}, &operatorv1.OperatorStatus{}, "", nil
}

func (noopOperatorClient) UpdateOperatorSpec(ctx context.Context, oldResourceVersion string, in *operatorv1.OperatorSpec) (out *operatorv1.OperatorSpec, newResourceVersion string, err error) {
	return in, "", nil
}

func (noopOperatorClient) UpdateOperatorStatus(ctx context.Context, oldResourceVersion string, in *operatorv1.OperatorStatus) (out *operatorv1.OperatorStatus, err error) {
	return in, nil
}
