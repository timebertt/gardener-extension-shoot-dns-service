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
	"encoding/json"
	"reflect"

	"github.com/gardener/gardener/pkg/apis/extensions/v1alpha1"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dnsapi "github.com/gardener/external-dns-management/pkg/apis/dns/v1alpha1"

	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/apis"
	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/apis/helper"
	wireapi "github.com/gardener/gardener-extension-shoot-dns-service/pkg/apis/v1alpha1"
)

////////////////////////////////////////////////////////////////////////////////
// state update handling

type StateHandler struct {
	client client.Client
	ctx context.Context
	ext *v1alpha1.Extension
	state *apis.DNSState
	modified bool
}

func NewStateHandler(ctx context.Context, client client.Client, ext *v1alpha1.Extension) (*StateHandler, error) {
	state, err:=helper.GetExtensionState(ext)
	if err != nil {
		return nil, err
	}
	return &StateHandler{
		client: client,
		ctx:    ctx,
		ext:    ext,
		state:  state,
	}, nil
}

func (s *StateHandler) EnsureEntries(entries []*dnsapi.DNSEntry) bool {
	mod := false
	names := sets.String{}
	for _, entry := range entries {
		mod = s.EnsureEntryFor(entry)
		names.Insert(entry.Name)
	}
	if len(entries) != len(s.state.Entries) {
		for i, e := range s.state.Entries {
			if !names.Has(e.Name) {
				mod = true
				s.state.Entries = append(s.state.Entries[:i], s.state.Entries[i+1:]...)
			}
		}
	}
	s.modified=s.modified||mod
	return mod
}

func (s *StateHandler) EnsureEntryDeleted(name string) bool {
	for i, e := range s.state.Entries {
		if e.Name == name {
			s.state.Entries = append(s.state.Entries[:i], s.state.Entries[i+1:]...)
			s.modified=true
			return true
		}
	}
	return false
}

func (s *StateHandler) EnsureEntryFor(entry *dnsapi.DNSEntry) bool {
	for _, e := range s.state.Entries {
		if e.Name == entry.Name {
			mod := false
			if !reflect.DeepEqual(&e.Spec, &entry.Spec) {
				mod = true
				e.Spec = entry.Spec.DeepCopy()
			}
			if !reflect.DeepEqual(&e.Annotations, &entry.Annotations) {
				mod = true
				e.Annotations = CopyMap(entry.Annotations)
			}
			if !reflect.DeepEqual(&e.Labels, &entry.Labels) {
				mod = true
				e.Labels = CopyMap(entry.Labels)
			}
			s.modified=s.modified||mod
			return mod
		}
	}

	e := &apis.DNSEntry{
		Name:        entry.Name,
		Labels:      CopyMap(entry.Labels),
		Annotations: CopyMap(entry.Annotations),
		Spec:        entry.Spec.DeepCopy(),
	}
	s.modified=true
	s.state.Entries = append(s.state.Entries, e)
	return true
}

func (s *StateHandler) Update() error {
	if s.modified {
		wire := &wireapi.DNSState{}
		wire.APIVersion = wireapi.SchemeGroupVersion.String()
		wire.Kind = wireapi.DNSStateKind
		err := helper.Scheme.Convert(s.state, wire, nil)
		if err != nil {
			return err
		}
		s.ext.Status.State.Raw, err = json.Marshal(wire)
		err = s.client.Update(s.ctx, s.ext)
		return err
	}
	return nil
}

