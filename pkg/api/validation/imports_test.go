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

package validation_test

import (
	"github.com/gardener/virtual-garden/pkg/api"
	. "github.com/gardener/virtual-garden/pkg/api/validation"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/utils/pointer"
)

var _ = Describe("Imports", func() {
	Describe("#ValidateImports", func() {
		var (
			obj *api.Imports
		)

		BeforeEach(func() {
			obj = &api.Imports{
				HostingCluster: api.HostingCluster{
					Kubeconfig:             "abc",
					Namespace:              "foo",
					InfrastructureProvider: "gcp",
				},
				VirtualGarden: api.VirtualGarden{
					ETCD: &api.ETCD{
						Backup: &api.ETCDBackup{
							InfrastructureProvider: api.InfrastructureProviderGCP,
							Region:                 "foo",
							BucketName:             "bar",
							CredentialsRef:         "baz",
						},
					},
				},
				Credentials: map[string]api.Credentials{
					"baz": {
						Type: api.InfrastructureProviderGCP,
						Data: map[string]string{"foo": "bar"},
					},
				},
			}
		})

		It("should pass for a valid configuration", func() {
			Expect(ValidateImports(obj)).To(BeEmpty())
		})

		Context("hosting cluster", func() {
			It("should fail for an invalid configuration", func() {
				obj.HostingCluster.Kubeconfig = ""
				obj.HostingCluster.Namespace = ""
				obj.HostingCluster.InfrastructureProvider = ""

				Expect(ValidateImports(obj)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("hostingCluster.kubeconfig"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("hostingCluster.namespace"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeNotSupported),
						"Field": Equal("hostingCluster.infrastructureProvider"),
					})),
				))
			})
		})

		Context("virtual garden", func() {
			Context("ETCD", func() {
				It("should pass when no ETCD settings are configured", func() {
					obj.VirtualGarden.ETCD = &api.ETCD{}
					Expect(ValidateImports(obj)).To(BeEmpty())
				})

				It("should fail for invalid configuration", func() {
					obj.VirtualGarden.ETCD = &api.ETCD{
						StorageClassName: pointer.StringPtr(""),
						Backup:           &api.ETCDBackup{},
					}

					Expect(ValidateImports(obj)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("virtualGarden.etcd.storageClassName"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeNotSupported),
							"Field": Equal("virtualGarden.etcd.backup.infrastructureProvider"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("virtualGarden.etcd.backup.region"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("virtualGarden.etcd.backup.bucketName"),
						})),
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeRequired),
							"Field": Equal("virtualGarden.etcd.backup.credentialsRef"),
						})),
					))
				})

				It("should fail when credentials ref is invalid", func() {
					obj.VirtualGarden.ETCD.Backup.CredentialsRef = "baz2"

					Expect(ValidateImports(obj)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("virtualGarden.etcd.backup.credentialsRef"),
						})),
					))
				})

				It("should fail when credentials ref points to credentials of different type", func() {
					obj.VirtualGarden.ETCD.Backup.CredentialsRef = "wrong"
					obj.Credentials["wrong"] = api.Credentials{Type: "type", Data: map[string]string{"foo": "bar"}}

					Expect(ValidateImports(obj)).To(ConsistOf(
						PointTo(MatchFields(IgnoreExtras, Fields{
							"Type":  Equal(field.ErrorTypeInvalid),
							"Field": Equal("virtualGarden.etcd.backup.credentialsRef"),
						})),
					))
				})
			})

			Context("KubeAPIServer", func() {
				Context("SNI", func() {
					It("should pass when no SNI is configured", func() {
						obj.VirtualGarden.KubeAPIServer = &api.KubeAPIServer{Exposure: &api.KubeAPIServerExposure{}}
						Expect(ValidateImports(obj)).To(BeEmpty())
					})

					It("should pass for a valid SNI configuration", func() {
						obj.VirtualGarden.KubeAPIServer = &api.KubeAPIServer{
							Exposure: &api.KubeAPIServerExposure{
								SNI: &api.SNI{
									Hostnames: []string{"foo.com"},
									DNSClass:  pointer.StringPtr("bar"),
									TTL:       pointer.Int32Ptr(62),
								},
							},
						}
						Expect(ValidateImports(obj)).To(BeEmpty())
					})

					DescribeTable("should fail for invalid SNI configuration",
						func(sni *api.SNI) {
							obj.VirtualGarden.KubeAPIServer = &api.KubeAPIServer{Exposure: &api.KubeAPIServerExposure{SNI: sni}}
							Expect(ValidateImports(obj)).To(ConsistOf(
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeRequired),
									"Field": Equal("virtualGarden.exposure.sni.hostnames"),
								})),
								PointTo(MatchFields(IgnoreExtras, Fields{
									"Type":  Equal(field.ErrorTypeInvalid),
									"Field": Equal("virtualGarden.exposure.sni.ttl"),
								})),
							))
						},

						Entry("no hostnames, ttl to low", &api.SNI{TTL: pointer.Int32Ptr(42)}),
						Entry("no hostnames, ttl to high", &api.SNI{TTL: pointer.Int32Ptr(1000)}),
					)
				})
			})
		})

		Context("credentials", func() {
			It("should fail for an invalid configuration", func() {
				obj.Credentials["foo"] = api.Credentials{}

				Expect(ValidateImports(obj)).To(ConsistOf(
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("credentials.foo.type"),
					})),
					PointTo(MatchFields(IgnoreExtras, Fields{
						"Type":  Equal(field.ErrorTypeRequired),
						"Field": Equal("credentials.foo.data"),
					})),
				))
			})
		})
	})
})
