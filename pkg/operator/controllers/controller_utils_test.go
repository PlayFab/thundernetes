package controllers

import (
	"fmt"
	"sort"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

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
		It("should return true for a ready and schedulable node", func() {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(IsNodeReadyAndSchedulable(node)).To(BeTrue())
		})
		It("should return false for an unschedulable node", func() {
			node := &corev1.Node{
				Spec: corev1.NodeSpec{Unschedulable: true},
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(IsNodeReadyAndSchedulable(node)).To(BeFalse())
		})
		It("should return false for a node with no Ready condition", func() {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
					},
				},
			}
			Expect(IsNodeReadyAndSchedulable(node)).To(BeFalse())
		})
		It("should return false for a node with Ready=False", func() {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeReady, Status: corev1.ConditionFalse},
					},
				},
			}
			Expect(IsNodeReadyAndSchedulable(node)).To(BeFalse())
		})
		It("should return true for a node with multiple conditions including Ready=True", func() {
			node := &corev1.Node{
				Status: corev1.NodeStatus{
					Conditions: []corev1.NodeCondition{
						{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse},
						{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse},
						{Type: corev1.NodeReady, Status: corev1.ConditionTrue},
					},
				},
			}
			Expect(IsNodeReadyAndSchedulable(node)).To(BeTrue())
		})
		It("should return a random string of correct length", func() {
			s := randString(10)
			Expect(len(s)).To(Equal(10))
			s2 := randString(0)
			Expect(len(s2)).To(Equal(0))
		})
		It("should return only lowercase and digit characters in randString", func() {
			s := randString(100)
			Expect(s).To(MatchRegexp("^[a-z0-9]+$"))
		})
		It("should return 0 for empty state in getValueByState", func() {
			gs := &mpsv1alpha1.GameServer{}
			Expect(getValueByState(gs)).To(Equal(0))
		})
		It("should return 1 for Initializing state in getValueByState", func() {
			gs := &mpsv1alpha1.GameServer{
				Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateInitializing},
			}
			Expect(getValueByState(gs)).To(Equal(1))
		})
		It("should return 2 for StandingBy state in getValueByState", func() {
			gs := &mpsv1alpha1.GameServer{
				Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateStandingBy},
			}
			Expect(getValueByState(gs)).To(Equal(2))
		})
		It("should return 3 for Active state in getValueByState", func() {
			gs := &mpsv1alpha1.GameServer{
				Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateActive},
			}
			Expect(getValueByState(gs)).To(Equal(3))
		})
		It("should return 3 for Crashed state in getValueByState", func() {
			gs := &mpsv1alpha1.GameServer{
				Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateCrashed},
			}
			Expect(getValueByState(gs)).To(Equal(3))
		})
		It("should sort GameServers by state using ByState", func() {
			gameServers := ByState{
				{Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateActive}},
				{Status: mpsv1alpha1.GameServerStatus{State: ""}},
				{Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateStandingBy}},
				{Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateInitializing}},
				{Status: mpsv1alpha1.GameServerStatus{State: mpsv1alpha1.GameServerStateCrashed}},
			}
			sort.Sort(gameServers)
			Expect(gameServers[0].Status.State).To(Equal(mpsv1alpha1.GameServerState("")))
			Expect(gameServers[1].Status.State).To(Equal(mpsv1alpha1.GameServerStateInitializing))
			Expect(gameServers[2].Status.State).To(Equal(mpsv1alpha1.GameServerStateStandingBy))
			// Active and Crashed both have value 3, so they can be in either order
			states := []mpsv1alpha1.GameServerState{gameServers[3].Status.State, gameServers[4].Status.State}
			Expect(states).To(ContainElement(mpsv1alpha1.GameServerStateActive))
			Expect(states).To(ContainElement(mpsv1alpha1.GameServerStateCrashed))
		})
		It("should create a pod for a Linux GameServer with correct properties", func() {
			gs := testGenerateGameServer("test-build", "test-build-id", "default", "test-gs-linux")
			gs.Spec.Template.Spec.Containers[0].Ports[0].HostPort = 20000
			pod := NewPodForGameServer(gs, "initcontainer-linux:latest", "initcontainer-win:latest")
			Expect(pod.Name).To(Equal(gs.Name))
			Expect(pod.Namespace).To(Equal(gs.Namespace))
			Expect(pod.Spec.RestartPolicy).To(Equal(corev1.RestartPolicyNever))
			// verify labels from template are preserved
			Expect(pod.Labels["label1"]).To(Equal("value1"))
			Expect(pod.Labels["label2"]).To(Equal("value2"))
			// verify thundernetes labels
			Expect(pod.Labels[LabelBuildID]).To(Equal("test-build-id"))
			Expect(pod.Labels[LabelBuildName]).To(Equal("test-build"))
			Expect(pod.Labels[LabelOwningGameServer]).To(Equal("test-gs-linux"))
			Expect(pod.Labels[LabelOwningOperator]).To(Equal("thundernetes"))
			// verify annotations from template are preserved
			Expect(pod.Annotations["annotation1"]).To(Equal("value1"))
			Expect(pod.Annotations["annotation2"]).To(Equal("value2"))
			// verify init container uses Linux image
			Expect(len(pod.Spec.InitContainers)).To(Equal(1))
			Expect(pod.Spec.InitContainers[0].Image).To(Equal("initcontainer-linux:latest"))
			// verify data volume exists
			foundVolume := false
			for _, v := range pod.Spec.Volumes {
				if v.Name == DataVolumeName {
					foundVolume = true
				}
			}
			Expect(foundVolume).To(BeTrue())
			// verify data volume mount on container uses Linux path
			foundMount := false
			for _, vm := range pod.Spec.Containers[0].VolumeMounts {
				if vm.Name == DataVolumeName && vm.MountPath == DataVolumeMountPath {
					foundMount = true
				}
			}
			Expect(foundMount).To(BeTrue())
		})
		It("should create a pod for a Windows GameServer with Windows paths", func() {
			gs := testGenerateGameServer("test-build", "test-build-id", "default", "test-gs-win")
			gs.Spec.Template.Spec.NodeSelector = map[string]string{"kubernetes.io/os": "windows"}
			gs.Spec.Template.Spec.Containers[0].Ports[0].HostPort = 20001
			pod := NewPodForGameServer(gs, "initcontainer-linux:latest", "initcontainer-win:latest")
			// verify init container uses Windows image
			Expect(pod.Spec.InitContainers[0].Image).To(Equal("initcontainer-win:latest"))
			// verify data volume mount on init container uses Windows path
			Expect(pod.Spec.InitContainers[0].VolumeMounts[0].MountPath).To(Equal(DataVolumeMountPathWin))
			// verify data volume mount on container uses Windows path
			foundWinMount := false
			for _, vm := range pod.Spec.Containers[0].VolumeMounts {
				if vm.Name == DataVolumeName && vm.MountPath == DataVolumeMountPathWin {
					foundWinMount = true
				}
			}
			Expect(foundWinMount).To(BeTrue())
		})
		It("should create a pod that preserves template annotations and labels", func() {
			gs := testGenerateGameServer("build-x", "build-id-x", "default", "gs-annot-test")
			gs.Spec.Template.Spec.Containers[0].Ports[0].HostPort = 20002
			pod := NewPodForGameServer(gs, "init-linux", "init-win")
			Expect(pod.Annotations).To(HaveKeyWithValue("annotation1", "value1"))
			Expect(pod.Annotations).To(HaveKeyWithValue("annotation2", "value2"))
			Expect(pod.Labels).To(HaveKeyWithValue("label1", "value1"))
			Expect(pod.Labels).To(HaveKeyWithValue("label2", "value2"))
		})
		It("should create a GameServer from a GameServerBuild with correct properties", func() {
			client := testNewSimpleK8sClient()
			pr, err := NewPortRegistry(client, &mpsv1alpha1.GameServerList{}, 20000, 20100, 1, false, ctrl.Log.WithName("test"))
			Expect(err).ToNot(HaveOccurred())
			gsb := testGenerateGameServerBuild("test-build-gsb", "default", "build-id-gsb", 2, 4, false)
			gs, err := NewGameServerForGameServerBuild(&gsb, pr)
			Expect(err).ToNot(HaveOccurred())
			Expect(gs).ToNot(BeNil())
			// verify labels
			Expect(gs.Labels[LabelBuildID]).To(Equal("build-id-gsb"))
			Expect(gs.Labels[LabelBuildName]).To(Equal("test-build-gsb"))
			// verify namespace
			Expect(gs.Namespace).To(Equal("default"))
			// verify owner reference
			Expect(len(gs.OwnerReferences)).To(Equal(1))
			Expect(gs.OwnerReferences[0].Kind).To(Equal(GameServerBuildKind))
			Expect(gs.OwnerReferences[0].Name).To(Equal("test-build-gsb"))
			// verify spec fields
			Expect(gs.Spec.BuildID).To(Equal("build-id-gsb"))
			Expect(gs.Spec.TitleID).To(Equal("test-title-id"))
			Expect(gs.Spec.PortsToExpose).To(Equal([]int32{80}))
			// verify that a host port was assigned
			Expect(gs.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(BeNumerically(">=", int32(20000)))
			Expect(gs.Spec.Template.Spec.Containers[0].Ports[0].HostPort).To(BeNumerically("<=", int32(20100)))
			// verify name has the build name prefix
			Expect(gs.Name).To(HavePrefix(fmt.Sprintf("%s-", gsb.Name)))
		})
	})
})
