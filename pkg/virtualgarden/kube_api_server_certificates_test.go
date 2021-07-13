// Copyright (c) 2021 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package virtualgarden

import (
	"context"
	lsv1alpha1 "github.com/gardener/landscaper/apis/core/v1alpha1"
	"github.com/gardener/virtual-garden/pkg/api"
	"github.com/gardener/virtual-garden/pkg/provider"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Deploy Item Controller Reconcile Test", func() {

	It("Should create api server secrets", func() {
		ctx := context.Background()
		defer ctx.Done()

		namespace := v1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: "test01"},
		}
		err := testenv.Client.Create(ctx, &namespace)
		Expect(err).To(BeNil())

		imports := getImports()

		infrastructureProvider, err := provider.NewInfrastructureProvider(api.InfrastructureProviderGCP)
		Expect(err).To(BeNil())

		operation := &operation{
			client:                 testenv.Client,
			log:                    testenv.Logger,
			infrastructureProvider: infrastructureProvider,
			backupProvider:         nil,
			namespace:              namespace.Name,
			imports:                &imports,
			exports:                api.Exports{},
			imageRefs:              api.ImageRefs{},
		}

		Expect(err).To(BeNil())

		checksums := make(map[string]string)
		err = operation.deployKubeAPIServerCertificates(ctx, "ourTestLoadBalancer", checksums)

		// check secrets
		objectKey := client.ObjectKey{Name: KubeApiServerSecretNameAggregatorCACertificate, Namespace: namespace.Name}
		secret := &v1.Secret{}
		err = testenv.Client.Get(ctx, objectKey, secret)
		Expect(err).To(BeNil())
	})
})

func getImports() api.Imports {
	return api.Imports{
		Cluster:        lsv1alpha1.Target{},
		HostingCluster: api.HostingCluster{},
		VirtualGarden: api.VirtualGarden{
			ETCD: nil,
			KubeAPIServer: &api.KubeAPIServer{
				Replicas:                 0,
				SNI:                      nil,
				DnsAccessDomain:          "com.our.test",
				GardenerControlplane:     api.GardenerControlplane{},
				AuditWebhookConfig:       api.AuditWebhookConfig{},
				AuditWebhookBatchMaxSize: "",
				SeedAuthorizer:           api.SeedAuthorizer{},
				HVPAEnabled:              false,
				HVPA:                     nil,
				EventTTL:                 nil,
				OidcIssuerURL:            nil,
				AdditionalVolumeMounts:   nil,
				AdditionalVolumes:        nil,
				HorizontalPodAutoscaler:  nil,
			},
			DeleteNamespace:   false,
			PriorityClassName: "",
		},
	}
}
