package replication

import (
	"reflect"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/runtime/inject"

	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/gardener/gardener-extension-shoot-dns-service/pkg/apis"
	controllerconfig "github.com/gardener/gardener-extension-shoot-dns-service/pkg/controller/config"
)

type reconciler struct {
	logger logr.Logger
	client client.Client

	controllerConfig controllerconfig.DNSServiceConfig
}

// NewReconciler creates a new reconcile.Reconciler that reconciles
// Extension resources of Gardener's `extensions.gardener.cloud` API group.
func NewReconciler(name string, controllerConfig controllerconfig.DNSServiceConfig) reconcile.Reconciler {
	logger := log.Log.WithName(name)
	return &reconciler{
		logger:           logger,
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

func (r *reconciler) Reconcile(req reconcile.Request) (reconcile.Result, error) {
	result := reconcile.Result{}
	return result, nil
}

func (r *reconciler) ensureEntries(state *apis.DNSState, entries []*apis.DNSEntry) bool {
	mod:=false
	names:=sets.String{}
	for _, entry := range entries {
		mod=r.ensureEntryFor(state, entry)
		names.Insert(entry.Name)
	}
	if len(entries) != len(state.Entries) {
		for i, e := range state.Entries {
			if !names.Has(e.Name) {
				mod=true
				state.Entries=append(state.Entries[:i], state.Entries[i+1:]...)
			}
		}
	}
	return mod
}

func (r *reconciler) ensureEntryDeleted(state *apis.DNSState, name string) bool {
	for i, e := range state.Entries {
		if e.Name ==name {
			state.Entries=append(state.Entries[:i], state.Entries[i+1:]...)
			return true
		}
	}
	return false
}

func (r *reconciler) ensureEntryFor(state *apis.DNSState, entry *apis.DNSEntry) bool {
	for _, e := range state.Entries {
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
			return mod
		}
	}

	e := &apis.DNSEntry{
		Name:        entry.Name,
		Labels:      CopyMap(entry.Labels),
		Annotations: CopyMap(entry.Annotations),
		Spec:        entry.Spec.DeepCopy(),
	}
	state.Entries = append(state.Entries, e)
	return true
}

func CopyMap(m map[string]string) map[string]string {
	if m == nil {
		return nil
	}
	r := map[string]string{}
	for k, v := range m {
		r[k] = v
	}
	return r
}
