package controllers

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/rand"
	"strconv"
	"strings"
	"time"

	mpsv1alpha1 "github.com/playfab/thundernetes/pkg/operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	InitContainerName = "initcontainer"

	GameServerKind      = "GameServer"
	GameServerBuildKind = "GameServerBuild"

	DataVolumeName         = "gsdkdata"
	DataVolumeMountPath    = "/gsdkdata"
	DataVolumeMountPathWin = "c:\\gsdkdata"
	RandStringSize         = 5

	LabelBuildID          = "BuildID"
	LabelBuildName        = "BuildName"
	LabelOwningGameServer = "mps.playfab.com/OwningGameServer"
	LabelOwningOperator   = "mps.playfab.com/OwningOperator"
	LabelNodeName         = "NodeName"

	GsdkConfigFile    = DataVolumeMountPath + "/Config/gsdkConfig.json"
	GsdkConfigFileWin = DataVolumeMountPathWin + "\\Config\\gsdkConfig.json"

	LogDirectory    = DataVolumeMountPath + "/GameLogs/"
	LogDirectoryWin = DataVolumeMountPathWin + "\\GameLogs\\"

	CertificatesDirectory    = DataVolumeMountPath + "/GameCertificates"
	CertificatesDirectoryWin = DataVolumeMountPathWin + "\\GameCertificates"

	GameSharedContentDirectory    = DataVolumeMountPath + "/GameSharedContent"
	GameSharedContentDirectoryWin = DataVolumeMountPathWin + "\\GameSharedContent"

	DaemonSetPort int32 = 56001

	LabelGameServerNode string = "mps.playfab.com/gameservernode"
)

func init() {
	rand.Seed(time.Now().UTC().UnixNano()) //randomize name creation
}

// generateName generates a random string concatenated with prefix and a dash
func generateName(prefix string) string {
	return prefix + "-" + randString(RandStringSize)
}

// randString creates a random string with lowercase characters
func randString(n int) string {
	letters := []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// getStateDuration determine whether to use an existing saved time variables or the current time for state duration
func getStateDuration(endTime *metav1.Time, startTime *metav1.Time) float64 {
	// If the end time state is missing, use the current time
	if endTime == nil {
		return math.Abs(float64(time.Since(startTime.Time).Milliseconds()))
	}
	return math.Abs(float64(endTime.Time.Sub(startTime.Time).Milliseconds()))
}

// GetNodeDetails returns the Public IP of the node and the node age in days
// if the Node does not have a Public IP, method returns the internal one
func GetNodeDetails(ctx context.Context, r client.Reader, nodeName string) (string, string, int, error) {
	log := log.FromContext(ctx)
	var node corev1.Node
	if err := r.Get(ctx, client.ObjectKey{Name: nodeName}, &node); err != nil {
		return "", "", 0, err
	}

	nodeAgeInDays := int(time.Since(node.CreationTimestamp.Time).Hours() / 24)

	for _, x := range node.Status.Addresses {
		if x.Type == corev1.NodeExternalIP {
			return nodeName, x.Address, nodeAgeInDays, nil
		}
	}
	log.Info(fmt.Sprintf("Node with name %s does not have a Public IP, will try to return the internal IP", nodeName))
	// externalIP not found, try InternalIP
	for _, x := range node.Status.Addresses {
		if x.Type == corev1.NodeInternalIP {
			return nodeName, x.Address, nodeAgeInDays, nil
		}
	}

	return nodeName, "", 0, fmt.Errorf("node %s does not have a Public or Internal IP", nodeName)
}

// NewGameServerForGameServerBuild creates a GameServer for a GameServerBuild
func NewGameServerForGameServerBuild(gsb *mpsv1alpha1.GameServerBuild, portRegistry *PortRegistry) (*mpsv1alpha1.GameServer, error) {
	gs := &mpsv1alpha1.GameServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      generateName(gsb.Name),
			Namespace: gsb.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gsb, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    GameServerBuildKind,
				}),
			},
			Labels: map[string]string{LabelBuildID: gsb.Spec.BuildID, LabelBuildName: gsb.Name},
		},
		Spec: mpsv1alpha1.GameServerSpec{
			// we're doing a DeepCopy since we modify the hostPort
			Template:      *gsb.Spec.Template.DeepCopy(),
			BuildID:       gsb.Spec.BuildID,
			TitleID:       gsb.Spec.TitleID,
			PortsToExpose: gsb.Spec.PortsToExpose,
			BuildMetadata: gsb.Spec.BuildMetadata,
		},
		// we don't create any status since we have the .Status subresource enabled
	}
	// get host ports
	// we assume that each portToExpose exists only once in the GameServer PodSpec.Containers.Ports.ContainerPort(s)
	// so we ask for len(gsb.Spec.PortsToExpose) ports
	hostPorts, err := portRegistry.GetNewPorts(gs.Namespace, gs.Name, len(gsb.Spec.PortsToExpose))
	j := 0
	if err != nil {
		return nil, err
	}
	// assigning host ports for all the containers in the Template.Spec
	for i := 0; i < len(gs.Spec.Template.Spec.Containers); i++ {
		container := gs.Spec.Template.Spec.Containers[i]
		for i := 0; i < len(container.Ports); i++ {
			if sliceContainsPortToExpose(gsb.Spec.PortsToExpose, container.Ports[i].ContainerPort) {
				container.Ports[i].HostPort = hostPorts[j]
				// if the user has specified that they want to use the host's network, we override the container port
				if gs.Spec.Template.Spec.HostNetwork {
					container.Ports[i].ContainerPort = hostPorts[j]
				}
				j++ // increase the hostPort index so on the next iteration (if any) we will use the next hostPort
			}
		}
	}

	return gs, nil
}

// NewPodForGameServer returns a Kubernetes Pod struct for a specified GameServer
// Pod has the same name as the GameServer
// It also sets a label called "GameServer" with the value of the corresponding GameServer resource
func NewPodForGameServer(gs *mpsv1alpha1.GameServer, initContainerImageLinux, initContainerImageWin string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      gs.Name, // same Name as the GameServer
			Namespace: gs.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(gs, schema.GroupVersionKind{
					Group:   mpsv1alpha1.GroupVersion.Group,
					Version: mpsv1alpha1.GroupVersion.Version,
					Kind:    GameServerKind,
				}),
			},
		},
		Spec: gs.Spec.Template.Spec,
	}

	// if the pod is to be run on windows
	isWindows := pod.Spec.NodeSelector["kubernetes.io/os"] == "windows"

	// copy Labels and Annotations from Pod Template
	pod.ObjectMeta.Annotations = gs.Spec.Template.Annotations
	pod.ObjectMeta.Labels = gs.Spec.Template.Labels

	// initialize the Labels map if it's nil, so we can add thundernetes Labels
	if pod.ObjectMeta.Labels == nil {
		pod.ObjectMeta.Labels = make(map[string]string)
	}

	// add thundernetes Labels
	pod.ObjectMeta.Labels[LabelBuildID] = gs.Spec.BuildID
	pod.ObjectMeta.Labels[LabelBuildName] = gs.Labels[LabelBuildName]
	pod.ObjectMeta.Labels[LabelOwningGameServer] = gs.Name
	pod.ObjectMeta.Labels[LabelOwningOperator] = "thundernetes"

	// following methods should be called in this exact order
	setPodRestartPolicyToNever(pod)
	createDataVolumeOnPod(pod)
	// attach data volume and env for all containers in the Pod
	for i := 0; i < len(pod.Spec.Containers); i++ {
		attachDataVolumeOnContainer(&pod.Spec.Containers[i], isWindows)
		pod.Spec.Containers[i].Env = append(pod.Spec.Containers[i].Env, getGameServerEnvVariables(gs, isWindows)...)
	}

	var initContainerImage string = initContainerImageLinux
	if isWindows {
		initContainerImage = initContainerImageWin
	}
	attachInitContainer(gs, pod, initContainerImage, isWindows)

	return pod
}

// setPodRestartPolicyToNever sets the Pod's restart policy to Never
func setPodRestartPolicyToNever(pod *corev1.Pod) {
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever
}

// attachInitContainer attaches the init container to the GameServer Pod
func attachInitContainer(gs *mpsv1alpha1.GameServer, pod *corev1.Pod, initContainerImage string, isWindows bool) {
	dataVolumeMountPath := DataVolumeMountPath
	if isWindows {
		dataVolumeMountPath = DataVolumeMountPathWin
	}
	initcontainer := corev1.Container{
		Name:            InitContainerName,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Image:           initContainerImage,
		Env:             getInitContainerEnvVariables(gs, isWindows),
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      DataVolumeName,
				MountPath: dataVolumeMountPath,
			},
		},
	}
	pod.Spec.InitContainers = append(pod.Spec.InitContainers, initcontainer)
}

// createDataVolumeOnPod creates a Volume that will be mounted to the GameServer Pod
// The init container writes to this volume whereas the GameServer container reads from it (the GSDK methods)
func createDataVolumeOnPod(pod *corev1.Pod) {
	dataDir := corev1.Volume{
		Name: DataVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	}
	pod.Spec.Volumes = append(pod.Spec.Volumes, dataDir)
}

// attachDataVolumeOnContainer attaches the data volume to the specified container
func attachDataVolumeOnContainer(container *corev1.Container, isWindows bool) {
	dataVolumeMountPath := DataVolumeMountPath
	if isWindows {
		dataVolumeMountPath = DataVolumeMountPathWin
	}
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      DataVolumeName,
		MountPath: dataVolumeMountPath,
	})
}

// getInitContainerEnvVariables returns the environment variables for the init container
func getInitContainerEnvVariables(gs *mpsv1alpha1.GameServer, isWindows bool) []corev1.EnvVar {
	gsdkConfigFile := GsdkConfigFile
	gameSharedContentDirectory := GameSharedContentDirectory
	certificatesDirectory := CertificatesDirectory
	logDirectory := LogDirectory
	if isWindows {
		gsdkConfigFile = GsdkConfigFileWin
		gameSharedContentDirectory = GameSharedContentDirectoryWin
		certificatesDirectory = CertificatesDirectoryWin
		logDirectory = LogDirectoryWin
	}
	envList := []corev1.EnvVar{
		{
			Name:  "HEARTBEAT_ENDPOINT_PORT",
			Value: fmt.Sprintf("%d", DaemonSetPort),
		},
		{
			Name:  "GSDK_CONFIG_FILE",
			Value: gsdkConfigFile,
		},
		{
			Name:  "PF_SHARED_CONTENT_FOLDER",
			Value: gameSharedContentDirectory,
		},
		{
			Name:  "CERTIFICATE_FOLDER",
			Value: certificatesDirectory,
		},
		{
			Name:  "PF_SERVER_LOG_DIRECTORY",
			Value: logDirectory,
		},
		{
			Name: "PF_VM_ID",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "spec.nodeName",
				},
			},
		},
		{
			Name: "PF_NODE_INTERNAL_IP",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "status.hostIP",
				},
			},
		},
		{
			Name:  "PF_GAMESERVER_NAME", // this becomes SessionHostId in gsdkConfig.json file
			Value: gs.Name,              // GameServer.Name is the same as Pod.Name
		},
		{
			Name:  "PF_GAMESERVER_NAMESPACE",
			Value: gs.Namespace,
		},
	}

	var b bytes.Buffer
	// get game ports
	for _, container := range gs.Spec.Template.Spec.Containers {
		for _, port := range container.Ports {
			if port.HostPort > 0 { // hostPort has been set, this means that this port is in the portsToExpose array
				containerPort := strconv.Itoa(int(port.ContainerPort))
				hostPort := strconv.Itoa(int(port.HostPort))
				b.WriteString(port.Name + "," + containerPort + "," + hostPort + "?")
			}
		}
	}

	envList = append(envList, corev1.EnvVar{
		Name:  "PF_GAMESERVER_PORTS",
		Value: strings.TrimSuffix(b.String(), "?"),
	})

	var buildMetada string
	for _, metadataItem := range gs.Spec.BuildMetadata {
		buildMetada += metadataItem.Key + "," + metadataItem.Value + "?"
	}
	envList = append(envList, corev1.EnvVar{
		Name:  "PF_GAMESERVER_BUILD_METADATA",
		Value: strings.TrimSuffix(buildMetada, "?"),
	})

	return envList
}

// ger getGameServerEnvVariables returns the environment variables for the GameServer container
func getGameServerEnvVariables(gs *mpsv1alpha1.GameServer, isWindows bool) []corev1.EnvVar {
	gsdkConfigFile := GsdkConfigFile
	logDirectory := LogDirectory
	if isWindows {
		gsdkConfigFile = GsdkConfigFileWin
		logDirectory = LogDirectoryWin
	}
	envList := []corev1.EnvVar{
		{
			Name:  "PF_GAMESERVER_NAME",
			Value: gs.Name,
		},
		{
			Name:  "GSDK_CONFIG_FILE",
			Value: gsdkConfigFile,
		},
		{
			Name:  "PF_GAMESERVER_NAMESPACE",
			Value: gs.Namespace,
		},
		{
			Name:  "PF_BUILD_ID",
			Value: gs.Spec.BuildID,
		},
		{
			Name:  "PF_TITLE_ID",
			Value: gs.Spec.TitleID,
		},
		{
			Name:  "PF_SERVER_LOG_DIRECTORY",
			Value: logDirectory,
		},
	}

	return envList
}

// sliceContainsPortToExpose returns true if the port is contained in the portsToExpose slice
func sliceContainsPortToExpose(portsToExpose []int32, port int32) bool {
	for _, item := range portsToExpose {
		if item == port {
			return true
		}
	}
	return false
}

// containsString returns true if the specific string value is contained in the slice
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// getContainerHostPortTuples returns a concatenated list of hostPort:containerPort tuples
func getContainerHostPortTuples(pod *corev1.Pod) string {
	var ports strings.Builder
	for _, container := range pod.Spec.Containers {
		for _, portInfo := range container.Ports {
			ports.WriteString(fmt.Sprintf("%d:%d,", portInfo.ContainerPort, portInfo.HostPort))
		}
	}
	return strings.TrimSuffix(ports.String(), ",")
}

// IsNodeReadyAndSchedulable returns true if the node is ready and schedulable
func IsNodeReadyAndSchedulable(node *corev1.Node) bool {
	if !node.Spec.Unschedulable {
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				return true
			}
		}
	}
	return false
}

// isNodeGameServerNode returns true if Node has the Label mps.playfab.com/gameservernode=true set
// this Label should be set when the cluster contains a specific Node Pool/Group for GameServers
func isNodeGameServerNode(node *corev1.Node) bool {
	return node.Labels != nil && node.Labels[LabelGameServerNode] == "true"
}

// ByState is a slice of GameServers
type ByState []mpsv1alpha1.GameServer

// Len is the number of elements in the collection
func (gs ByState) Len() int { return len(gs) }

// Less helps sort the GameServer slice by the following order
// first are the Initializing GameServers and then the StandingBy
// everything else goes last
func (gs ByState) Less(i, j int) bool {
	return getValueByState(&gs[i]) < getValueByState(&gs[j])
}

// Swap swaps the elements at the passed indexes
func (gs ByState) Swap(i, j int) { gs[i], gs[j] = gs[j], gs[i] }

// getValueByState returns the value of the state of the GameServer
// to help in sorting the array
// we want to delete the GameServers that are in empty ("") state first (since they might have Pods Pending or waiting to start)
// then the ones on Initializing state
// and lastly the ones on StandingBy state
// GameServers that have crashed or Terminated are taken care of when the GameServerBuild controller starts
func getValueByState(gs *mpsv1alpha1.GameServer) int {
	switch gs.Status.State {
	case "":
		return 0
	case mpsv1alpha1.GameServerStateInitializing:
		return 1
	case mpsv1alpha1.GameServerStateStandingBy:
		return 2
	default:
		return 3
	}
}
