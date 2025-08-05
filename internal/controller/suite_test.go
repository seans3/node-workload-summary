/*
Copyright 2025.

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

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	uxv1alpha1 "github.com/seans3/node-workload-summary/api/v1alpha1"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var (
	ctx       context.Context
	cancel    context.CancelFunc
	testEnv   *envtest.Environment
	cfg       *rest.Config
	k8sClient client.Client
)

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.TODO())

	var err error
	err = uxv1alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())
	err = appsv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Retrieve the first found binary directory to allow running tests from IDEs
	if getFirstFoundEnvTestBinaryDir() != "" {
		testEnv.BinaryAssetsDirectory = getFirstFoundEnvTestBinaryDir()
	}

	// cfg is defined in this file globally.
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	cancel()
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

// getFirstFoundEnvTestBinaryDir locates the first binary in the specified path.
// ENVTEST-based tests depend on specific binaries, usually located in paths set by
// controller-runtime. When running tests directly (e.g., via an IDE) without using
// the 'BinaryAssetsDirectory' must be explicitly configured.
//
// This function streamlines the process by finding the required binaries, similar to
// setting the 'KUBEBUILDER_ASSETS' environment variable. To ensure the binaries are
// properly set up, run 'make setup-envtest' beforehand.
func getFirstFoundEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")
	entries, err := os.ReadDir(basePath)
	if err != nil {
		logf.Log.Error(err, "Failed to read directory", "path", basePath)
		return ""
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}
	return ""
}

var _ = Describe("FindRootWorkload", func() {
	var (
		fakeClient    client.Client
		workloadTypes map[schema.GroupKind]bool
	)

	BeforeEach(func() {
		workloadTypes = map[schema.GroupKind]bool{
			{Group: "apps", Kind: "Deployment"}:  true,
			{Group: "apps", Kind: "StatefulSet"}: true,
		}
	})

	It("should return the pod itself if it has no owner", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "pod-a",
				Namespace: "default",
			},
		}
		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pod).Build()

		root, err := FindRootWorkload(ctx, fakeClient, pod, workloadTypes)
		Expect(err).NotTo(HaveOccurred())
		Expect(root.GetName()).To(Equal("pod-a"))
	})

	It("should find the Deployment as the root workload", func() {
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-deployment",
				Namespace: "default",
				UID:       "dep-uid",
			},
		}
		replicaSet := &appsv1.ReplicaSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-replicaset",
				Namespace: "default",
				UID:       "rs-uid",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment")),
				},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "my-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(replicaSet, appsv1.SchemeGroupVersion.WithKind("ReplicaSet")),
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pod, replicaSet, deployment).Build()

		root, err := FindRootWorkload(ctx, fakeClient, pod, workloadTypes)
		Expect(err).NotTo(HaveOccurred())
		Expect(root.GetName()).To(Equal("my-deployment"))
		Expect(root.GetObjectKind().GroupVersionKind().Kind).To(Equal("Deployment"))
	})

	It("should return the pod if the owner is not found", func() {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "orphan-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: "apps/v1",
						Kind:       "ReplicaSet",
						Name:       "non-existent-rs",
						UID:        "non-existent-uid",
						Controller: func(b bool) *bool { return &b }(true),
					},
				},
			},
		}
		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pod).Build()

		root, err := FindRootWorkload(ctx, fakeClient, pod, workloadTypes)
		Expect(err).NotTo(HaveOccurred())
		Expect(root.GetName()).To(Equal("orphan-pod"))
	})

	It("should handle deep ownership chains and stop at the workload", func() {
		customResource := &uxv1alpha1.NodeSummarizer{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "custom-resource",
				Namespace: "default",
				UID:       "cr-uid",
			},
		}
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deep-deployment",
				Namespace: "default",
				UID:       "deep-dep-uid",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(customResource, uxv1alpha1.GroupVersion.WithKind("NodeSummarizer")),
				},
			},
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "deep-pod",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment")),
				},
			},
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(pod, deployment, customResource).Build()

		root, err := FindRootWorkload(ctx, fakeClient, pod, workloadTypes)
		Expect(err).NotTo(HaveOccurred())
		Expect(root.GetName()).To(Equal("deep-deployment"))
	})

	It("should return an error for excessively deep chains", func() {
		owners := make([]client.Object, MaxOwnerTraversalDepth+1)
		for i := 0; i <= MaxOwnerTraversalDepth; i++ {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "deep-pod-" + strconv.Itoa(i),
					Namespace: "default",
					UID:       types.UID("pod-uid-" + strconv.Itoa(i)),
				},
			}
			if i > 0 {
				prevOwner := owners[i-1]
				pod.OwnerReferences = []metav1.OwnerReference{
					*metav1.NewControllerRef(prevOwner.(metav1.Object), corev1.SchemeGroupVersion.WithKind("Pod")),
				}
			}
			owners[i] = pod
		}

		fakeClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(owners...).Build()
		_, err := FindRootWorkload(ctx, fakeClient, owners[MaxOwnerTraversalDepth], workloadTypes)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("ownership chain is too deep"))
	})
})

var _ = Describe("WorkloadSummary Reconciler", func() {
	Context("When reconciling a WorkloadSummary", func() {
		const resourceName = "test-deployment"
		ctx := context.Background()
		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating a Deployment")
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: func() *int32 { i := int32(3); return &i }(),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

			By("creating a ReplicaSet")
			rs := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-replicaset",
					Namespace: "default",
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(deployment, appsv1.SchemeGroupVersion.WithKind("Deployment")),
					},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"app": "test"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{"app": "test"},
						},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "test",
									Image: "nginx",
								},
							},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, rs)).To(Succeed())

			By("creating Pods")
			for i := 0; i < 3; i++ {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("test-pod-%d", i),
						Namespace: "default",
						Labels:    map[string]string{"app": "test"},
						OwnerReferences: []metav1.OwnerReference{
							*metav1.NewControllerRef(rs, appsv1.SchemeGroupVersion.WithKind("ReplicaSet")),
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "test",
								Image: "nginx",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			}

			By("creating a WorkloadSummary")
			workloadSummary := &uxv1alpha1.WorkloadSummary{
				ObjectMeta: metav1.ObjectMeta{
					Name:      resourceName,
					Namespace: "default",
				},
			}
			Expect(k8sClient.Create(ctx, workloadSummary)).To(Succeed())

			By("creating a WorkloadSummarizer")
			workloadSummarizer := &uxv1alpha1.WorkloadSummarizer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-summarizer",
					Namespace: "default",
				},
				Spec: uxv1alpha1.WorkloadSummarizerSpec{
					WorkloadTypes: []uxv1alpha1.WorkloadType{
						{
							Group:   "apps",
							Kind:    "Deployment",
							Version: "v1",
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, workloadSummarizer)).To(Succeed())
		})

		AfterEach(func() {
			By("cleaning up the resources")
			Expect(k8sClient.DeleteAllOf(ctx, &uxv1alpha1.WorkloadSummary{}, client.InNamespace("default"))).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &corev1.Pod{}, client.InNamespace("default"), client.MatchingLabels{"app": "test"})).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &appsv1.ReplicaSet{}, client.InNamespace("default"))).To(Succeed())
			Expect(k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace("default"))).To(Succeed())
		})

		It("should correctly count the pods for the workload", func() {
			controllerReconciler := &WorkloadSummaryReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())

			workloadSummary := &uxv1alpha1.WorkloadSummary{}
			Eventually(func() (int, error) {
				err := k8sClient.Get(ctx, typeNamespacedName, workloadSummary)
				if err != nil {
					return 0, err
				}
				return workloadSummary.Status.PodCount, nil
			}, "10s", "1s").Should(Equal(3))
			Expect(workloadSummary.Status.ShortType).To(Equal("dep"))
			Expect(workloadSummary.Status.LongType).To(Equal("apps/v1.Deployment"))
		})
	})
})
