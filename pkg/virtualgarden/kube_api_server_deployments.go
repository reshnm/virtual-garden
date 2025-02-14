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
	_ "embed"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	KubeAPIServerDeploymentNameAPIServer         = Prefix + "-kube-apiserver"
	KubeAPIServerDeploymentNameControllerManager = Prefix + "-kube-controller-manager"
)

func (o *operation) deleteDeployments(ctx context.Context) error {
	o.log.Infof("Deleting deployments for the kube-apiserver")

	for _, name := range []string{
		KubeAPIServerDeploymentNameAPIServer,
		KubeAPIServerDeploymentNameControllerManager,
	} {
		deployment := o.emptyDeployment(name)
		if err := o.client.Delete(ctx, deployment); client.IgnoreNotFound(err) != nil {
			return err
		}
	}
	return nil
}

func (o *operation) deployKubeAPIServerDeployment(ctx context.Context, checksums map[string]string, staticTokenHealthCheck string) error {
	o.log.Infof("Deploying deployment %s", KubeAPIServerDeploymentNameAPIServer)

	deployment := o.emptyDeployment(KubeAPIServerDeploymentNameAPIServer)

	// compute particular values
	apiServerImports := o.imports.VirtualGarden.KubeAPIServer

	replicas := pointer.Int32Ptr(int32(apiServerImports.Replicas))

	annotations := o.computeApiServerAnnotations(checksums)

	command := o.getAPIServerCommand()

	// create/update
	_, err := controllerutil.CreateOrUpdate(ctx, o.client, deployment, func() error {
		deployment.ObjectMeta.Labels = kubeAPIServerLabels()

		deployment.Spec = appsv1.DeploymentSpec{
			RevisionHistoryLimit: pointer.Int32Ptr(0),
			Replicas:             replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: kubeAPIServerLabels(),
			},

			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: annotations,
					Labels: map[string]string{
						LabelKeyApp:                                                   Prefix,
						LabelKeyComponent:                                             "kube-apiserver",
						"networking.gardener.cloud/to-dns":                            LabelValueAllowed,
						"networking.gardener.cloud/to-etcd":                           LabelValueAllowed,
						"networking.gardener.cloud/to-gardener-apiserver":             LabelValueAllowed,
						"networking.gardener.cloud/to-gardener-admission-controller":  LabelValueAllowed, // needed for webhooks
						"networking.gardener.cloud/to-identity":                       LabelValueAllowed,
						"networking.gardener.cloud/to-ingress":                        LabelValueAllowed, // needed for communication to identity
						"networking.gardener.cloud/to-terminal-controller-manager":    LabelValueAllowed, // needed for webhooks
						"networking.gardener.cloud/to-gardenlogin-controller-manager": LabelValueAllowed, // needed for webhooks
						"networking.gardener.cloud/to-world":                          LabelValueAllowed, // GCP puts IP tables on nodes that allow for local routing, for other cloudproviders this is needed
					},
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										TopologyKey: "kubernetes.io/hostname",
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      LabelKeyApp,
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{Prefix},
												},
												{
													Key:      LabelKeyComponent,
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"kube-apiserver"},
												},
											},
										},
									},
								},
							},
						},
					},
					AutomountServiceAccountToken: pointer.BoolPtr(false),
					ServiceAccountName:           KubeAPIServerServiceName,
					PriorityClassName:            o.imports.VirtualGarden.PriorityClassName,
					Containers: []corev1.Container{
						{
							Name:            kubeAPIServerContainerName,
							Image:           o.imageRefs.KubeAPIServerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Command:         command,
							Lifecycle: &corev1.Lifecycle{
								PreStop: &corev1.Handler{
									Exec: &corev1.ExecAction{
										Command: []string{"sh", "-c", "sleep 5"},
									},
								},
							},
							LivenessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:        "/livez",
										Port:        intstr.IntOrString{Type: intstr.Int, IntVal: 443},
										Scheme:      corev1.URISchemeHTTPS,
										HTTPHeaders: o.getAPIServerHeaders(staticTokenHealthCheck),
									},
								},
								InitialDelaySeconds: 15,
								TimeoutSeconds:      15,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								Handler: corev1.Handler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:        "/readyz",
										Port:        intstr.IntOrString{Type: intstr.Int, IntVal: 443},
										Scheme:      corev1.URISchemeHTTPS,
										HTTPHeaders: o.getAPIServerHeaders(staticTokenHealthCheck),
									},
								},
								InitialDelaySeconds: 10,
								TimeoutSeconds:      15,
								PeriodSeconds:       30,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							TerminationMessagePath:   "/dev/termination-log",
							TerminationMessagePolicy: corev1.TerminationMessageReadFile,
							Ports: []corev1.ContainerPort{
								{
									Name:          "https",
									ContainerPort: 443,
									Protocol:      "TCP",
								},
							},
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("2"),
									corev1.ResourceMemory: resource.MustParse("2000Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("600m"),
									corev1.ResourceMemory: resource.MustParse("512Mi"),
								},
							},
							VolumeMounts: o.getAPIServerVolumeMounts(),
						}, // end first and only container
					}, // end Containers
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(30),
					Volumes:                       o.getAPIServerVolumes(),
				},
			},
		}
		return nil
	})

	return err
}

func (o *operation) computeApiServerAnnotations(checksums map[string]string) map[string]string {
	annotations := o.addChecksumsToAnnotations(checksums, []string{
		ChecksumKeyKubeAPIServerAuditPolicyConfig,
		ChecksumKeyKubeAPIServerEncryptionConfig,
		ChecksumKeyKubeAggregatorCA,
		ChecksumKeyKubeAggregatorClient,
		ChecksumKeyKubeAPIServerCA,
		ChecksumKeyKubeAPIServerServer,
		ChecksumKeyKubeAPIServerAuditWebhookConfig,
		ChecksumKeyKubeAPIServerAuthWebhookConfig,
		ChecksumKeyKubeAPIServerOidcAuthenticationWebhookConfig,
		ChecksumKeyKubeAPIServerStaticToken,
		ChecksumKeyKubeAPIServerAdmissionConfig,
	})
	return annotations
}

func (o *operation) addChecksumsToAnnotations(checksums map[string]string, keys []string) map[string]string {
	annotations := make(map[string]string)

	for _, key := range keys {
		value, found := checksums[key]
		if found {
			annotations[key] = value
		}
	}

	return annotations
}

func (o *operation) getAPIServerCommand() []string {
	command := []string{}
	command = append(command, "/usr/local/bin/kube-apiserver")
	if o.isWebhookEnabled() {
		command = append(command, "--admission-control-config-file=/etc/gardener-apiserver/admission/configuration.yaml")
	}
	command = append(command, "--allow-privileged=true")
	command = append(command, "--anonymous-auth=false")
	command = append(command, "--audit-policy-file=/etc/kube-apiserver/audit/audit-policy.yaml")
	if o.getAuditWebhookBatchMaxSize() != "" {
		command = append(command, fmt.Sprintf("--audit-webhook-batch-max-size=%s", o.getAuditWebhookBatchMaxSize()))
	}
	if len(o.getAPIServerAuditWebhookConfig()) > 0 {
		command = append(command, "--audit-webhook-config-file=/etc/kube-apiserver/auditwebhook/audit-webhook-config.yaml")
	}

	if o.isOidcWebhookAuthenticatorEnabled() {
		command = append(command, "--authentication-token-webhook-config-file=/etc/kube-apiserver/authentication-webhook/kubeconfig.yaml")
		command = append(command, "--authentication-token-webhook-cache-ttl=0")
	}

	if o.isSeedAuthorizerEnabled() {
		command = append(command, "--authorization-mode=RBAC,Webhook")
		command = append(command, "--authorization-webhook-config-file=/etc/kube-apiserver/auth-webhook/config.yaml")
		command = append(command, "--authorization-webhook-cache-authorized-ttl=0")
		command = append(command, "--authorization-webhook-cache-unauthorized-ttl=0")
	} else {
		command = append(command, "--authorization-mode=RBAC")
	}
	command = append(command, "--client-ca-file=/srv/kubernetes/ca/ca.crt")
	command = append(command, "--disable-admission-plugins=PersistentVolumeLabel")
	command = append(command, "--enable-admission-plugins=Priority,NamespaceLifecycle,LimitRanger,PodSecurityPolicy,ServiceAccount,NodeRestriction,DefaultStorageClass,DefaultTolerationSeconds,ResourceQuota,StorageObjectInUseProtection,MutatingAdmissionWebhook,ValidatingAdmissionWebhook")
	command = append(command, "--enable-aggregator-routing=true")
	command = append(command, "--enable-bootstrap-token-auth=true")
	if o.hasEncryptionConfig() {
		command = append(command, "--encryption-provider-config=/etc/kube-apiserver/encryption/encryption-config.yaml")
	}
	command = append(command, "--etcd-cafile=/srv/kubernetes/etcd/ca/ca.crt")
	command = append(command, "--etcd-certfile=/srv/kubernetes/etcd/client/tls.crt")
	command = append(command, "--etcd-keyfile=/srv/kubernetes/etcd/client/tls.key")
	command = append(command, fmt.Sprintf("--etcd-servers=https://virtual-garden-etcd-main-client.%s.svc:2379",
		o.namespace))
	command = append(command, fmt.Sprintf("--etcd-servers-overrides=/events#https://virtual-garden-etcd-events-client.%s.svc:2379,coordination.k8s.io/leases#https://virtual-garden-etcd-events-client.%s.svc:2379",
		o.namespace, o.namespace))
	if o.getAPIServerEventTTL() != "" {
		command = append(command, fmt.Sprintf("--event-ttl=%s", o.getAPIServerEventTTL()))
	}
	command = append(command, "--insecure-port=0")
	command = append(command, "--kubelet-preferred-address-types=InternalIP,Hostname,ExternalIP")
	command = append(command, "--livez-grace-period=1m")
	command = append(command, fmt.Sprintf("--max-mutating-requests-inflight=%d",
		o.imports.VirtualGarden.KubeAPIServer.GetMaxMutatingRequestsInflight(400)))
	command = append(command, fmt.Sprintf("--max-requests-inflight=%d",
		o.imports.VirtualGarden.KubeAPIServer.GetMaxRequestsInflight(800)))
	if o.getAPIServerOIDCIssuerURL() != nil {
		command = append(command, "--oidc-client-id=kube-kubectl")
		command = append(command, "--oidc-groups-claim=groups")
		command = append(command, fmt.Sprintf("--oidc-issuer-url=%s", *o.getAPIServerOIDCIssuerURL()))
		command = append(command, "--oidc-username-claim=email")
	}
	command = append(command, fmt.Sprintf("--profiling=%t", o.imports.VirtualGarden.KubeAPIServer.Profiling))
	command = append(command, "--proxy-client-cert-file=/srv/kubernetes/aggregator/tls.crt")
	command = append(command, "--proxy-client-key-file=/srv/kubernetes/aggregator/tls.key")
	command = append(command, "--requestheader-client-ca-file=/srv/kubernetes/aggregator-ca/ca.crt")
	command = append(command, "--requestheader-extra-headers-prefix=X-Remote-Extra-")
	command = append(command, "--requestheader-group-headers=X-Remote-Group")
	command = append(command, "--requestheader-username-headers=X-Remote-User")
	command = append(command, "--secure-port=443")
	command = append(command, "--service-account-key-file=/srv/kubernetes/service-account-key/service_account.key")
	command = append(command, "--service-account-signing-key-file=/srv/kubernetes/service-account-key/service_account.key")
	command = append(command, fmt.Sprintf("--service-account-issuer=https://gardener.%s", o.imports.VirtualGarden.KubeAPIServer.DnsAccessDomain))
	command = append(command, "--service-cluster-ip-range=100.64.0.0/13")
	command = append(command, "--shutdown-delay-duration=20s")
	command = append(command, "--tls-cert-file=/srv/kubernetes/apiserver/tls.crt")
	command = append(command, "--tls-cipher-suites=TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,TLS_RSA_WITH_AES_128_CBC_SHA,TLS_RSA_WITH_AES_256_CBC_SHA,TLS_RSA_WITH_AES_128_GCM_SHA256,TLS_RSA_WITH_AES_256_GCM_SHA384,TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA")
	command = append(command, "--tls-private-key-file=/srv/kubernetes/apiserver/tls.key")
	if o.isSNIEnabled() {
		command = append(command, fmt.Sprintf("--tls-sni-cert-key=/srv/kubernetes/sni-tls/tls.crt,/srv/kubernetes/sni-tls/tls.key:%s", o.getSNIHostname()))
	}
	command = append(command, "--token-auth-file=/srv/kubernetes/token/static_tokens.csv")
	command = append(command, "--v=2")
	command = append(command, "--watch-cache-sizes=secrets#500,configmaps#500")

	return command
}

func (o *operation) getAPIServerAuditWebhookConfig() string {
	return o.imports.VirtualGarden.KubeAPIServer.AuditWebhookConfig.Config
}

func (o *operation) getAuditWebhookBatchMaxSize() string {
	// comes from landscape.yaml
	// components.gardener.controlplane.values.apiserver.audit.webhook.batchMaxSize: "30"
	return o.imports.VirtualGarden.KubeAPIServer.AuditWebhookBatchMaxSize
}

func (o *operation) isSeedAuthorizerEnabled() bool {
	return o.imports.VirtualGarden.KubeAPIServer != nil && o.imports.VirtualGarden.KubeAPIServer.SeedAuthorizer.Enabled
}

func (o *operation) isOidcWebhookAuthenticatorEnabled() bool {
	return o.imports.VirtualGarden.KubeAPIServer != nil && o.imports.VirtualGarden.KubeAPIServer.OidcWebhookAuthenticator.Enabled
}

func (o *operation) hasEncryptionConfig() bool {
	return true
}

func (o *operation) getAPIServerEventTTL() string {
	if o.imports.VirtualGarden.KubeAPIServer.EventTTL == nil {
		return "24h"
	}

	return *o.imports.VirtualGarden.KubeAPIServer.EventTTL
}

func (o *operation) getAPIServerOIDCIssuerURL() *string {
	return o.imports.VirtualGarden.KubeAPIServer.OidcIssuerURL
}

func (o *operation) isSNIEnabled() bool {
	return o.imports.VirtualGarden.KubeAPIServer.SNI != nil
}

func (o *operation) getSNIHostname() string {
	return o.imports.VirtualGarden.KubeAPIServer.SNI.Hostname
}

func (o *operation) getAPIServerHeaders(staticTokenHealthCheck string) []corev1.HTTPHeader {
	return []corev1.HTTPHeader{
		{
			Name:  "Authorization",
			Value: "Bearer " + staticTokenHealthCheck,
		},
	}
}

func (o *operation) getAPIServerVolumeMounts() []corev1.VolumeMount {
	volumeMounts := []corev1.VolumeMount{}

	volumeMounts = append(volumeMounts, corev1.VolumeMount{
		Name:      volumeNameKubeAPIServerAuditPolicyConfig,
		MountPath: "/etc/kube-apiserver/audit",
	})

	if len(o.getAPIServerAuditWebhookConfig()) > 0 {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerAuditWebhookConfig,
			MountPath: "/etc/kube-apiserver/auditwebhook",
		})
	}

	if o.hasEncryptionConfig() {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerEncryptionConfig,
			MountPath: "/etc/kube-apiserver/encryption",
		})
	}

	if o.isSeedAuthorizerEnabled() {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerAuthWebhookConfig,
			MountPath: "/etc/kube-apiserver/auth-webhook",
		})
	}

	if o.isOidcWebhookAuthenticatorEnabled() {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerOidcAuthenticationWebhookConfig,
			MountPath: "/etc/kube-apiserver/authentication-webhook",
		})
	}

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerCA,
			MountPath: "/srv/kubernetes/ca",
		},
		corev1.VolumeMount{
			Name:      volumeNameCAETCD,
			MountPath: "/srv/kubernetes/etcd/ca",
		},
		corev1.VolumeMount{
			Name:      volumeNameCAFrontProxy,
			MountPath: "/srv/kubernetes/aggregator-ca",
		},
		corev1.VolumeMount{
			Name:      volumeNameETCDClientTLS,
			MountPath: "/srv/kubernetes/etcd/client",
		},
		corev1.VolumeMount{
			Name:      volumeNameKubeAPIServer,
			MountPath: "/srv/kubernetes/apiserver",
		},
		corev1.VolumeMount{
			Name:      volumeNameServiceAccountKey,
			MountPath: "/srv/kubernetes/service-account-key",
		},
		corev1.VolumeMount{
			Name:      volumeNameKubeAPIServerStaticToken,
			MountPath: "/srv/kubernetes/token",
		},
		corev1.VolumeMount{
			Name:      volumeNameKubeAggregator,
			MountPath: "/srv/kubernetes/aggregator",
		},
	)

	if o.isSNIEnabled() {
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      volumeNameSNITLS,
			MountPath: "/srv/kubernetes/sni-tls",
		})
	}

	// locations are taken from
	// https://github.com/golang/go/blob/1bb247a469e306c57a5e0eaba788efb8b3b1acef/src/crypto/x509/root_linux.go#L7-L15
	// we cannot be sure on which Node OS the Seed Cluster is running so, it's safer to mount them all

	volumeMounts = append(volumeMounts,
		corev1.VolumeMount{
			Name:      volumeNameFedora,
			MountPath: "/etc/pki/tls",
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      volumeNameCentos,
			MountPath: "/etc/pki/ca-trust/extracted/pem",
			ReadOnly:  true,
		},
		corev1.VolumeMount{
			Name:      volumeNameETCSSL,
			MountPath: "/etc/ssl",
			ReadOnly:  true,
		},
	)

	if o.isWebhookEnabled() {
		volumeMounts = append(volumeMounts,
			corev1.VolumeMount{
				Name:      volumeNameKubeAPIServerAdmissionConfig,
				MountPath: "/etc/gardener-apiserver/admission",
			},
			corev1.VolumeMount{
				Name:      volumeNameKubeAPIServerAdmissionKubeconfig,
				MountPath: "/var/run/secrets/admission-kubeconfig",
			},
			corev1.VolumeMount{
				Name:      volumeNameKubeAPIServerAdmissionTokens,
				MountPath: "/var/run/secrets/admission-tokens",
			},
		)
	}

	volumeMounts = append(volumeMounts, o.imports.VirtualGarden.KubeAPIServer.AdditionalVolumeMounts...)

	return volumeMounts
}

func (o *operation) getAPIServerVolumes() []corev1.Volume {
	volumes := []corev1.Volume{}

	if o.hasEncryptionConfig() {
		volumes = append(volumes, volumeWithSecretSource(volumeNameKubeAPIServerEncryptionConfig, KubeApiServerSecretNameEncryptionConfig))
	}

	if o.isSeedAuthorizerEnabled() {
		volumes = append(volumes, volumeWithSecretSource(volumeNameKubeAPIServerAuthWebhookConfig, KubeApiServerSecretNameAuthWebhookConfig))
	}

	if o.isOidcWebhookAuthenticatorEnabled() {
		volumes = append(volumes, volumeWithSecretSource(volumeNameKubeAPIServerOidcAuthenticationWebhookConfig, KubeApiServerSecretNameOidcAuthenticationWebhookConfig))
	}

	volumes = append(volumes, volumeWithConfigMapSource(volumeNameKubeAPIServerAuditPolicyConfig, KubeApiServerConfigMapAuditPolicy))

	if len(o.getAPIServerAuditWebhookConfig()) > 0 {
		volumes = append(volumes, volumeWithSecretSource(volumeNameKubeAPIServerAuditWebhookConfig, KubeApiServerSecretNameAuditWebhookConfig))
	}

	volumes = append(volumes,
		volumeWithSecretSource(volumeNameKubeAPIServerCA, KubeApiServerSecretNameApiServerCACertificate),
		volumeWithSecretSource(volumeNameCAETCD, "virtual-garden-etcd-ca"),
		volumeWithSecretSource(volumeNameCAFrontProxy, KubeApiServerSecretNameAggregatorCACertificate),
		volumeWithSecretSource(volumeNameKubeAPIServer, KubeApiServerSecretNameApiServerServerCertificate),
		volumeWithSecretSource(volumeNameETCDClientTLS, "virtual-garden-etcd-client"),
		volumeWithSecretSource(volumeNameKubeAPIServerStaticToken, KubeApiServerSecretNameStaticToken),
		volumeWithSecretSource(volumeNameServiceAccountKey, KubeApiServerSecretNameServiceAccountKey),
		volumeWithSecretSource(volumeNameKubeAggregator, KubeApiServerSecretNameAggregatorClientCertificate),
	)

	if o.isSNIEnabled() {
		volumes = append(volumes, volumeWithSecretSource(volumeNameSNITLS, o.imports.VirtualGarden.KubeAPIServer.SNI.SecretName))
	}

	if o.isWebhookEnabled() {
		volumes = append(volumes,
			volumeWithConfigMapSource(volumeNameKubeAPIServerAdmissionConfig, KubeApiServerConfigMapAdmission),
			volumeWithSecretSource(volumeNameKubeAPIServerAdmissionKubeconfig, KubeApiServerSecretNameAdmissionKubeconfig),
		)

		projections := []corev1.VolumeProjection{}
		controlplane := o.imports.VirtualGarden.KubeAPIServer.GardenerControlplane
		if controlplane.ValidatingWebhook.Token.Enabled {
			projections = append(projections, corev1.VolumeProjection{
				ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
					Audience:          controlplane.ValidatingWebhook.Token.Audience,
					ExpirationSeconds: pointer.Int64Ptr(controlplane.ValidatingWebhook.Token.ExpirationSeconds),
					Path:              "validating-webhook-token",
				},
			})
		}
		if o.imports.VirtualGarden.KubeAPIServer.GardenerControlplane.MutatingWebhook.Token.Enabled {
			projections = append(projections, corev1.VolumeProjection{
				ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
					Audience:          controlplane.MutatingWebhook.Token.Audience,
					ExpirationSeconds: pointer.Int64Ptr(controlplane.MutatingWebhook.Token.ExpirationSeconds),
					Path:              "mutating-webhook-token",
				},
			})
		}
		volumes = append(volumes,
			corev1.Volume{
				Name: volumeNameKubeAPIServerAdmissionTokens,
				VolumeSource: corev1.VolumeSource{
					Projected: &corev1.ProjectedVolumeSource{
						Sources: projections,
					},
				},
			},
		)
	}

	// locations are taken from
	// https://github.com/golang/go/blob/1bb247a469e306c57a5e0eaba788efb8b3b1acef/src/crypto/x509/root_linux.go#L7-L15
	// we cannot be sure on which Node OS the Seed Cluster is running so, it's safer to mount them all

	hostPathDirectoryOrCreate := corev1.HostPathDirectoryOrCreate
	volumes = append(volumes,
		corev1.Volume{
			Name: volumeNameFedora,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/pki/tls",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		corev1.Volume{
			Name: volumeNameCentos,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/pki/ca-trust/extracted/pem",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
		corev1.Volume{
			Name: volumeNameETCSSL,
			VolumeSource: corev1.VolumeSource{
				HostPath: &corev1.HostPathVolumeSource{
					Path: "/etc/ssl",
					Type: &hostPathDirectoryOrCreate,
				},
			},
		},
	)

	volumes = append(volumes, o.imports.VirtualGarden.KubeAPIServer.AdditionalVolumes...)

	return volumes
}

func (o *operation) emptyDeployment(name string) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: o.namespace}}
}
