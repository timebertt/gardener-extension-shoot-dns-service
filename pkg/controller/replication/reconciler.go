/*
 * Copyright 2020 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *       http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 */

package replication

import (
	"context"

	dnsapi "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/go-logr/logr"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	extapi "github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"

	controllerconfig "github.com/gardener/gardener-extension-shoot-dns-service/pkg/controller/config"
)

type reconciler struct {
	ctx    context.Context
	name   string
	logger logr.Logger
	client client.Client

	controllerConfig controllerconfig.DNSServiceConfig
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Extension resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(name string, controllerConfig controllerconfig.DNSServiceConfig) reconcile.Reconciler {
	return &reconciler{
		ctx:              context.Background(),
		name:             name,
		controllerConfig: controllerConfig,
	}
}

// InjectFunc enables dependency injection into the actuator.
func (r *reconciler) InjectFunc(f inject.Func) error {
	return nil
}

// InjectClient injects the controller runtime client into the reconciler.
func (r *reconciler) InjectClient(client client.Client) error {
	r.client = client
	return nil
}

// InjectClient injects the controller runtime client into the reconciler.
func (r *reconciler) InjectLogger(l logr.Logger) error {
	r.logger = l.WithName(r.name)
	return nil
}

////////////////////////////////////////////////////////////////////////////////
// entry reconcilation

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	result := reconcile.Result{}

	ext, err:=r.findExtension(req.Namespace)
	if err != nil {
	    return result, err
	}
	statehandler, err:=NewStateHandler(r.ctx, r.client, ext)
	if err != nil {
		return result, err
	}

	mod:=false
	entry:=&dnsapi.DNSEntry{}
	err=r.client.Get(r.ctx, req.NamespacedName, entry)
	if err!=nil {
		if !errors.IsNotFound(err) {
			return result, err
		}
		mod=r.delete(statehandler, req)
	}
	if entry.DeletionTimestamp!=nil {
		mod=r.delete(statehandler, req)
	} else {
		mod= r.reconcile(statehandler, entry)
	}
	if mod {
		return result, statehandler.Update()
	}
	return result, nil
}

func (r *reconciler) reconcile(statehandler *StateHandler, entry *dnsapi.DNSEntry) bool {
	return statehandler.EnsureEntryFor(entry)
}

func (r *reconciler) delete(statehandler *StateHandler, req reconcile.Request) bool {
	return statehandler.EnsureEntryDeleted(req.Name)
}

////////////////////////////////////////////////////////////////////////////////
// extension handling

func (r *reconciler) findExtension(namespace string) (*extapi.Extension, error) {
	return FindExtension(r.ctx, r.client, namespace)
}
