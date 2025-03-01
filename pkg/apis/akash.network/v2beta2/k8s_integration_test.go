//go:build k8s_integration

package v2beta2_test

import (
	"context"
	"fmt"
	"math/rand"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	atestutil "github.com/akash-network/node/testutil"

	clusterutil "github.com/akash-network/provider/cluster/util"
	crd "github.com/akash-network/provider/pkg/apis/akash.network/v2beta2"
	akashclient "github.com/akash-network/provider/pkg/client/clientset/versioned"
	mtestutil "github.com/akash-network/provider/testutil/manifest/v2beta2"
)

func TestWriteRead(t *testing.T) {
	ctx := context.Background()

	withNamespace(ctx, t, func(kcfg *rest.Config, ns string) {
		client, err := akashclient.NewForConfig(kcfg)
		require.NoError(t, err)

		for _, spec := range mtestutil.Generators {
			// ensure decode(encode(obj)) == obj

			lid := atestutil.LeaseID(t)
			group := spec.Generator.Group(t)

			csettings := crd.ClusterSettings{
				SchedulerParams: make([]*crd.SchedulerParams, len(group.Services)),
			}
			kmani, err := crd.NewManifest(ns, lid, &group, csettings)

			require.NoError(t, err, spec.Name)

			// save to k8s
			obj, err := client.AkashV2beta2().Manifests(ns).Create(ctx, kmani, metav1.CreateOptions{})
			require.NoError(t, err, spec.Name)

			// ensure created CRD has correct name
			assert.Equal(t, clusterutil.LeaseIDToNamespace(lid), obj.GetName(), spec.Name)

			// convert to akash-native objects and ensure no data corruption
			deployment, err := obj.Deployment()
			require.NoError(t, err, spec.Name)

			assert.Equal(t, lid, deployment.LeaseID(), spec.Name)
			assert.Equal(t, &group, deployment.ManifestGroup(), spec.Name)
		}
	})
}

func withNamespace(ctx context.Context, t testing.TB, fn func(*rest.Config, string)) {
	kcfg := kubeConfig(t)

	kc, err := kubernetes.NewForConfig(kcfg)
	require.NoError(t, err)

	// create a namespace with a random name and a searchable label.
	nsname := fmt.Sprintf("akash-test-ns-%v", rand.Uint64())
	_, err = kc.CoreV1().Namespaces().Create(ctx, &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsname,
			Labels: map[string]string{
				"akash.network/integration-test": "true",
			},
		},
	}, metav1.CreateOptions{})
	require.NoError(t, err)

	defer func() {
		// delete namespace
		err = kc.CoreV1().Namespaces().Delete(ctx, nsname, metav1.DeleteOptions{})
		require.NoError(t, err)
	}()

	// invoke callback
	fn(kcfg, nsname)

}

func kubeConfig(t testing.TB) *rest.Config {
	t.Helper()
	cfgPath := path.Join(homedir.HomeDir(), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", cfgPath)
	require.NoError(t, err)
	return config
}
