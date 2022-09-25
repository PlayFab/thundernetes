package controllers

import (
	"fmt"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Utilities tests", func() {
	Context("Testing Utilities", func() {
		It("should allocate hostPorts when creating game servers", func() {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "nginx",
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									ContainerPort: 80,
									HostPort:      123,
								},
								{
									Name:          "https",
									ContainerPort: 443,
									HostPort:      456,
								},
							},
						},
					},
				},
			}
			s := getContainerHostPortTuples(pod)
			Expect(s).To(Equal("80:123,443:456"))
		})
		It("should find if string is contained in the string slice", func() {
			Expect(containsString([]string{"foo"}, "foo")).To(BeTrue())
			Expect(containsString([]string{"foo"}, "bar")).To(BeFalse())
		})
		It("should find if containerName/portName tuple is contained in the PortToExpose slice", func() {
			p := []int32{
				5, 10, 15,
			}
			Expect(sliceContainsPortToExpose(p, 5)).To(BeTrue())
			Expect(sliceContainsPortToExpose(p, 10)).To(BeTrue())
			Expect(sliceContainsPortToExpose(p, 16)).To(BeFalse())
		})
		It("should return env variables for GameServer", func() {
			gs := &mpsv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gs",
					Namespace: "test-ns",
				},
				Spec: mpsv1alpha1.GameServerSpec{
					TitleID: "test-title",
					BuildID: "test-build",
				},
			}
			s := getGameServerEnvVariables(gs, false)
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_GAMESERVER_NAME", Value: "test-gs"})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_GAMESERVER_NAMESPACE", Value: "test-ns"})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_BUILD_ID", Value: "test-build"})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_TITLE_ID", Value: "test-title"})).To(BeTrue())
		})
		It("should return env variables for InitContainer", func() {
			gs := &mpsv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-GameServer",
				},
				Spec: mpsv1alpha1.GameServerSpec{
					TitleID: "test-title",
					BuildID: "test-build",
					PortsToExpose: []int32{
						80, 443,
					},
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name: "container1",
									Ports: []corev1.ContainerPort{
										{
											Name:          "port1",
											ContainerPort: 80,
											HostPort:      123,
										},
										{
											Name:          "port2",
											ContainerPort: 443,
											HostPort:      456,
										},
										{
											Name:          "port3",
											ContainerPort: 8080,
											// this is not on GameServer.PortsToExpose so there will be no HostPost
										},
									},
								},
							},
						},
					},
				},
			}

			s := getInitContainerEnvVariables(gs, false)
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "HEARTBEAT_ENDPOINT_PORT", Value: fmt.Sprintf("%d", DaemonSetPort)})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "GSDK_CONFIG_FILE", Value: GsdkConfigFile})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_SHARED_CONTENT_FOLDER", Value: GameSharedContentDirectory})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "CERTIFICATE_FOLDER", Value: CertificatesDirectory})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_SERVER_LOG_DIRECTORY", Value: LogDirectory})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_GAMESERVER_NAME", Value: gs.Name})).To(BeTrue())
			Expect(testVerifyEnv(s, corev1.EnvVar{Name: "PF_GAMESERVER_PORTS", Value: "port1,80,123?port2,443,456"})).To(BeTrue())
		})
		It("should attach data volume", func() {
			container := &corev1.Container{}
			attachDataVolumeOnContainer(container, false)
			Expect(container.VolumeMounts[len(container.VolumeMounts)-1]).To(BeEquivalentTo(corev1.VolumeMount{
				Name:      DataVolumeName,
				MountPath: DataVolumeMountPath,
			}))
		})
		It("should create data volume", func() {
			pod := &corev1.Pod{}
			createDataVolumeOnPod(pod)
			Expect(pod.Spec.Volumes[len(pod.Spec.Volumes)-1]).To(BeEquivalentTo(corev1.Volume{
				Name: DataVolumeName,
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			}))
		})
		It("should attach init container", func() {
			gs := &mpsv1alpha1.GameServer{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-gs",
					Namespace: "test-ns",
				},
				Spec: mpsv1alpha1.GameServerSpec{
					TitleID: "test-title",
					BuildID: "test-build",
				},
			}
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-pod",
				},
			}
			var testInitContainerImage string = "testInitContainerImage"
			attachInitContainer(gs, pod, testInitContainerImage, false)
			Expect(pod.Spec.InitContainers[len(pod.Spec.InitContainers)-1]).To(BeEquivalentTo(corev1.Container{
				Name:            InitContainerName,
				ImagePullPolicy: corev1.PullIfNotPresent,
				Image:           testInitContainerImage,
				Env:             getInitContainerEnvVariables(gs, false),
				VolumeMounts: []corev1.VolumeMount{
					{
						Name:      DataVolumeName,
						MountPath: DataVolumeMountPath,
					},
				},
			}))
		})
		It("shoud modify restart policy", func() {
			pod := &corev1.Pod{
				Spec: corev1.PodSpec{},
			}
			setPodRestartPolicyToNever(pod)
			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
		})
		It("should generate a random name with prefix", func() {
			prefix := "panathinaikos"
			s := generateName(prefix)
			Expect(s).To(HavePrefix(prefix))
			Expect(len(s)).To(BeNumerically(">", len(prefix)))
		})
		It("should check if a Node is a GameServer Node", func() {
			node := &corev1.Node{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						LabelGameServerNode: "true",
					},
				},
			}
			Expect(isNodeGameServerNode(node)).To(BeTrue())
			node.Labels[LabelGameServerNode] = "nottrue"
			Expect(isNodeGameServerNode(node)).To(BeFalse())
		})
	})
})
