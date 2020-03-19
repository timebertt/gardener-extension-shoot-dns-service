// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lifecycle

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1helper "github.com/gardener/gardener/pkg/apis/core/v1beta1/helper"

	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/controller/common"
	controllerconfig "github.com/gardener/gardener-extension-shoot-dns-service/pkg/controller/config"
	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/imagevector"
	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/service"

	"github.com/gardener/gardener-extensions/pkg/controller"
	"github.com/gardener/gardener-extensions/pkg/controller/extension"
	"github.com/gardener/gardener-extensions/pkg/util"
	extensionsv1alpha1 "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"github.com/gardener/gardener/pkg/chartrenderer"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/utils/chart"
	"github.com/gardener/gardener/pkg/utils/secrets"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	// ActuatorName is the name of the DNS Service actuator.
	ActuatorName = service.ServiceName + "-actuator"
	// SeedResourcesName is the name for resource describing the resources applied to the seed cluster.
	SeedResourcesName = service.ExtensionServiceName + "-seed"
	// ShootResourcesName is the name for resource describing the resources applied to the shoot cluster.
	ShootResourcesName = service.ExtensionServiceName + "-shoot"
	// KeptShootResourcesName is the name for resource describing the resources applied to the shoot cluster that should not be deleted.
	KeptShootResourcesName = service.ExtensionServiceName + "-shoot-keep"
)

// NewActuator returns an actuator responsible for Extension resources.
func NewActuator(config controllerconfig.DNSServiceConfig) extension.Actuator {
	return &actuator{
		Env: common.NewEnv(ActuatorName, config),
	}
}

type actuator struct {
	*common.Env
	applier    kubernetes.ChartApplier
	renderer   chartrenderer.Interface
	restConfig *rest.Config
}

// InjectConfig injects the rest config to this actuator.
func (a *actuator) InjectConfig(config *rest.Config) error {
	a.restConfig = config

	applier, err := kubernetes.NewChartApplierForConfig(a.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create chart applier: %v", err)
	}
	a.applier = applier

	renderer, err := chartrenderer.NewForConfig(a.restConfig)
	if err != nil {
		return fmt.Errorf("failed to create chart renderer: %v", err)
	}
	a.renderer = renderer

	return nil
}

// Reconcile the Extension resource.
func (a *actuator) Reconcile(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	cluster, err := controller.GetCluster(ctx, a.Client(), ex.Namespace)
	if err != nil {
		return err
	}

	// Shoots that don't specify a DNS domain or that are scheduled to a seed that is tainted with "DNS disabled"
	// don't get an DNS service
	if gardencorev1beta1helper.TaintsHave(cluster.Seed.Spec.Taints, gardencorev1beta1.SeedTaintDisableDNS) ||
		cluster.Shoot.Spec.DNS == nil {
		a.Info("DNS domain is not specified or the seed is tainted with 'disable-dns', therefore no shoot dns service is installed", "shoot", ex.Namespace)
		return a.Delete(ctx, ex)
	}

	if err := a.createShootResources(ctx, cluster, ex.Namespace); err != nil {
		return err
	}
	return a.createSeedResources(ctx, cluster, ex)
}

// Delete the Extension resource.
func (a *actuator) Delete(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	if err := a.deleteSeedResources(ctx, ex); err != nil {
		return err
	}
	return a.deleteShootResources(ctx, ex.Namespace)
}

func (a *actuator) createSeedResources(ctx context.Context, cluster *controller.Cluster, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.Namespace
	shootKubeconfig, err := a.createKubeconfig(ctx, namespace)
	if err != nil {
		return err
	}

	handler, err := common.NewStateHandler(a.Env, ex, true)
	if err != nil {
		return err
	}
	err = handler.Update()
	if err != nil {
		return err
	}

	chartValues := map[string]interface{}{
		"serviceName":         service.ServiceName,
		"replicas":            controller.GetReplicas(cluster, 1),
		"targetClusterSecret": shootKubeconfig.GetName(),
		"gardenId":            a.Config().GardenID,
		"shootId":             a.ShootId(namespace),
		"seedId":              a.Config().SeedID,
		"dnsClass":            a.Config().DNSClass,
		"podAnnotations": map[string]interface{}{
			"checksum/secret-kubeconfig": util.ComputeChecksum(shootKubeconfig.Data),
		},
	}

	chartValues, err = chart.InjectImages(chartValues, imagevector.ImageVector(), []string{service.ImageName})
	if err != nil {
		return fmt.Errorf("failed to find image version for %s: %v", service.ImageName, err)
	}

	a.Info("Component is being applied", "component", service.ExtensionServiceName, "namespace", namespace)
	return a.createManagedResource(ctx, namespace, SeedResourcesName, "seed", a.renderer, service.SeedChartName, chartValues, nil)
}

func (a *actuator) deleteSeedResources(ctx context.Context, ex *extensionsv1alpha1.Extension) error {
	namespace := ex.Namespace
	a.Info("Component is being deleted", "component", service.ExtensionServiceName, "namespace", namespace)

	if err := controller.DeleteManagedResource(ctx, a.Client(), namespace, SeedResourcesName); err != nil {
		return err
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := controller.WaitUntilManagedResourceDeleted(timeoutCtx, a.Client(), namespace, SeedResourcesName); err != nil {
		return err
	}

	secret := &corev1.Secret{}
	secret.SetName(service.SecretName)
	secret.SetNamespace(namespace)
	if err := a.Client().Delete(ctx, secret); client.IgnoreNotFound(err) != nil {
		return err
	}

	handler, err := common.NewStateHandler(a.Env, ex, true)
	if err != nil {
		return err
	}
	for _, item := range handler.Items() {
		if err := handler.Delete(item.Name); err != nil {
			return err
		}
	}
	return nil
}

func (a *actuator) createShootResources(ctx context.Context, cluster *controller.Cluster, namespace string) error {
	crd := &unstructured.Unstructured{}
	crd.SetAPIVersion("apiextensions.k8s.io/v1beta1")
	crd.SetKind("CustomResourceDefinition")
	if err := a.Client().Get(ctx, client.ObjectKey{Name: "dnsentries.dns.gardener.cloud"}, crd); err != nil {
		return errors.Wrap(err, "could not get crd dnsentries.dns.gardener.cloud")
	}
	crd.SetResourceVersion("")
	crd.SetUID("")
	crd.SetCreationTimestamp(metav1.Time{})
	crd.SetGeneration(0)
	if err := controller.CreateManagedResourceFromUnstructured(ctx, a.Client(), namespace, KeptShootResourcesName, "", []*unstructured.Unstructured{crd}, true, nil); err != nil {
		return errors.Wrapf(err, "could not create managed resource %s", KeptShootResourcesName)
	}

	renderer, err := util.NewChartRendererForShoot(cluster.Shoot.Spec.Kubernetes.Version)
	if err != nil {
		return errors.Wrap(err, "could not create chart renderer")
	}

	chartValues := map[string]interface{}{
		"userName":    service.UserName,
		"serviceName": service.ServiceName,
	}
	injectedLabels := map[string]string{controller.ShootNoCleanupLabel: "true"}

	return a.createManagedResource(ctx, namespace, ShootResourcesName, "", renderer, service.ShootChartName, chartValues, injectedLabels)
}

func (a *actuator) deleteShootResources(ctx context.Context, namespace string) error {
	if err := controller.DeleteManagedResource(ctx, a.Client(), namespace, ShootResourcesName); err != nil {
		return err
	}
	if err := controller.DeleteManagedResource(ctx, a.Client(), namespace, KeptShootResourcesName); err != nil {
		return err
	}

	timeoutCtx1, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := controller.WaitUntilManagedResourceDeleted(timeoutCtx1, a.Client(), namespace, ShootResourcesName); err != nil {
		return err
	}

	timeoutCtx2, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	return controller.WaitUntilManagedResourceDeleted(timeoutCtx2, a.Client(), namespace, KeptShootResourcesName)
}

func (a *actuator) createKubeconfig(ctx context.Context, namespace string) (*corev1.Secret, error) {
	certConfig := secrets.CertificateSecretConfig{
		Name:       service.SecretName,
		CommonName: service.UserName,
	}
	return util.GetOrCreateShootKubeconfig(ctx, a.Client(), certConfig, namespace)
}

func (a *actuator) createManagedResource(ctx context.Context, namespace, name, class string, renderer chartrenderer.Interface, chartName string, chartValues map[string]interface{}, injectedLabels map[string]string) error {
	return controller.CreateManagedResourceFromFileChart(
		ctx, a.Client(), namespace, name, class,
		renderer, filepath.Join(service.ChartsPath, chartName), chartName,
		chartValues, injectedLabels,
	)
}
