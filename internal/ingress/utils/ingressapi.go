package utils

import (
	"fmt"
	"testing"

	"github.com/pkg/errors"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/discovery"
)

type IngressAPI int

const (
	OtherAPI          IngressAPI = iota
	NetworkingV1      IngressAPI = iota
	NetworkingV1beta1 IngressAPI = iota
	ExtensionsV1beta1 IngressAPI = iota
)

func (ia IngressAPI) String() string {
	switch ia {
	case NetworkingV1:
		return networkingv1.SchemeGroupVersion.String()
	case NetworkingV1beta1:
		return networkingv1beta1.SchemeGroupVersion.String()
	case ExtensionsV1beta1:
		return extensionsv1beta1.SchemeGroupVersion.String()
	}
	return "unknown API"
}

// serverHasGVK returns true iff the Kubernetes API server supports the given resource kind at the given group-version.
func serverHasGVK(client discovery.ServerResourcesInterface, groupVersion, kind string) (bool, error) {
	list, err := client.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		return false, err
	}

	for _, elem := range list.APIResources {
		if elem.Kind == kind {
			return true, nil
		}
	}
	return false, nil
}

func NegotiateIngressAPI(client discovery.ServerResourcesInterface, allowedVersions []IngressAPI) (IngressAPI, error) {
	for _, candidate := range allowedVersions {
		if ok, err := serverHasGVK(client, candidate.String(), "Ingress"); err != nil {
			return OtherAPI, errors.Wrapf(err, "serverHasGVK(%v): ", candidate)
		} else if ok {
			return candidate, nil
		}
	}
	return OtherAPI, fmt.Errorf("no suitable Ingress API found, tried: %v", allowedVersions)
}

type fakeDiscoveryClient struct {
	discovery.ServerResourcesInterface

	results map[string]metav1.APIResourceList
	err     error
}

// ServerResourcesForGroupVersion returns the supported resources for a group and version.
func (fdc *fakeDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	resp := fdc.results[groupVersion]
	return &resp, fdc.err
}

func TestServerHasGVK(t *testing.T) {
	okClient := fakeDiscoveryClient{
		results: map[string]metav1.APIResourceList{
			"vegetables.k8s.io/v1": {APIResources: []metav1.APIResource{
				{Kind: "Potato"},
				{Kind: "Carrot"},
				{Kind: "Lettuce"},
			}},
			"fruits.k8s.io/v1": {APIResources: []metav1.APIResource{
				{Kind: "Apple"},
				{Kind: "Banana"},
				{Kind: "Pear"},
			}},
		},
	}

	errClient := fakeDiscoveryClient{
		err: errors.New("some fake error"),
	}

	for _, tt := range []struct {
		name   string
		client discovery.ServerResourcesInterface

		groupVersion, kind string

		wantResult bool
		wantErr    bool
	}{
		{
			name:         "positive case",
			client:       &okClient,
			groupVersion: "vegetables.k8s.io/v1",
			kind:         "Carrot",
			wantResult:   true,
		},
		{
			name:         "error",
			client:       &errClient,
			groupVersion: "vegetables.k8s.io/v1",
			kind:         "Carrot",
			wantErr:      true,
		},
		{
			name:         "gv has no such kind",
			client:       &okClient,
			groupVersion: "vegetables.k8s.io/v1",
			kind:         "Australia",
			wantResult:   false,
		},
		{
			name:         "has kind in another gv",
			client:       &okClient,
			groupVersion: "fruits.k8s.io/v1",
			kind:         "Potato",
			wantResult:   false,
		},
		{
			name:         "no such gv",
			client:       &okClient,
			groupVersion: "grains.k8s.io",
			kind:         "Wheat",
			wantResult:   false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			gotResult, gotErr := serverHasGVK(tt.client, tt.groupVersion, tt.kind)

			if gotResult != tt.wantResult {
				t.Errorf("serverHasGVK result: got %t, want %t", gotResult, tt.wantResult)
			}
			if (gotErr != nil) != tt.wantErr {
				t.Errorf("serverHasGVK: got error: %v, wanted error? %t", gotErr, tt.wantErr)
			}
		})
	}

}
