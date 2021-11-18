package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/util/retry"
)

// GsdkConfig is the configuration for the GSDK
// it will be written to the file that will be read by the GSDK running alongside the GameServer
type GsdkConfig struct {
	HeartbeatEndpoint        string                   `json:"heartbeatEndpoint"`
	SessionHostId            string                   `json:"sessionHostId"`
	VmId                     string                   `json:"vmId"`
	LogFolder                string                   `json:"logFolder"`
	CertificateFolder        string                   `json:"certificateFolder"`
	SharedContentFolder      string                   `json:"sharedContentFolder"`
	BuildMetadata            map[string]string        `json:"buildMetadata"`
	GamePorts                map[string]string        `json:"gamePorts"`
	PublicIpV4Address        string                   `json:"publicIpV4Address"`
	GameServerConnectionInfo GameServerConnectionInfo `json:"gameServerConnectionInfo"`
	ServerInstanceNumber     int                      `json:"serverInstanceNumber"` // Not used
	FullyQualifiedDomainName string                   `json:"fullyQualifiedDomainName"`
}

type GameServerConnectionInfo struct {
	PublicIpV4Address      string     `json:"publicIpV4Address"`
	GamePortsConfiguration []GamePort `json:"gamePortsConfiguration"`
}

type GamePort struct {
	Name                 string `json:"name"`
	ServerListeningPort  int    `json:"serverListeningPort"`
	ClientConnectionPort int    `json:"clientConnectionPort"`
}

var (
	heartbeatEndpoint       string
	gsdkConfigFilePath      string
	sharedContentFolderPath string
	certificateFolderPath   string
	serverLogPath           string
	gamePortsString         string
	sessionHostId           string
	crdNamespace            string
	nodeName                string
	logger                  *log.Entry
)

func main() {
	getGameServerNameNamespaceFromEnv()
	logger = log.WithFields(log.Fields{"GameServerName": sessionHostId, "GameServerNamespace": crdNamespace})

	getRestEnvVariables()

	gamePorts, gamePortConfiguration, err := parsePorts()
	if err != nil {
		logger.Fatalf("Could not parse game ports %s", err)
	}

	buildMetadata := parseBuildMetadata()

	nodeIpAddress := getPublicIpAddress()

	config := &GsdkConfig{
		HeartbeatEndpoint:   heartbeatEndpoint,
		SessionHostId:       sessionHostId,
		VmId:                nodeName,
		LogFolder:           serverLogPath,
		CertificateFolder:   certificateFolderPath,
		SharedContentFolder: sharedContentFolderPath,
		BuildMetadata:       buildMetadata,
		GamePorts:           gamePorts,
		PublicIpV4Address:   nodeIpAddress,
		GameServerConnectionInfo: GameServerConnectionInfo{
			PublicIpV4Address:      nodeIpAddress,
			GamePortsConfiguration: gamePortConfiguration,
		},
		FullyQualifiedDomainName: "NOT_APPLICABLE",
	}

	logger.Info("Marshalling to JSON")
	configJson, err := json.Marshal(config)
	handleError(err)

	logger.Info("Getting and creating folder(s)")
	folderPath := filepath.Dir(gsdkConfigFilePath)
	err = os.MkdirAll(folderPath, os.ModePerm)
	handleError(err)

	logger.Infof("Creating empty GSDK JSON file %s", gsdkConfigFilePath)
	f, err := os.Create(gsdkConfigFilePath)
	handleError(err)

	logger.Infof("Saving GSDK JSON to file %s", gsdkConfigFilePath)
	_, err = f.Write(configJson)
	handleError(err)
}

func getPublicIpAddress() string {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Fatal(err.Error())
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		logger.Fatal(err.Error())
	}

	var node *corev1.Node
	err = retry.OnError(retry.DefaultRetry, func(_ error) bool { return true }, func() error {
		node, err = clientset.CoreV1().Nodes().Get(context.Background(), nodeName, metav1.GetOptions{})
		if err != nil {
			logger.Warnf("Could not get node %s: %s", nodeName, err)
			return err
		}
		return nil
	})
	if err != nil {
		logger.Fatal(err.Error())
	}

	if node.Status.Addresses == nil {
		logger.Error("Node has no addresses")
		return "N/A"
	}

	var externalIp, internalIp string

	for _, address := range node.Status.Addresses {
		if address.Type == "ExternalIP" {
			externalIp = address.Address
		} else if address.Type == "InternalIP" {
			internalIp = address.Address
		}
	}

	if externalIp != "" {
		return externalIp
	}

	logger.Info("Node has no external IP address")

	if internalIp != "" {
		return internalIp
	}

	logger.Info("Node has no internal IP address either")
	return "N/A"
}

func parseBuildMetadata() map[string]string {
	buildMetadata := make(map[string]string)
	if os.Getenv("PF_GAMESERVER_BUILD_METADATA") != "" {
		metadata := os.Getenv("PF_GAMESERVER_BUILD_METADATA")
		s := strings.Split(metadata, "?")
		for _, s2 := range s {
			if s2 == "" {
				continue
			}
			s3 := strings.Split(s2, ",")
			buildMetadata[s3[0]] = s3[1]
		}
	}
	return buildMetadata
}

func parsePorts() (map[string]string, []GamePort, error) {
	// format is port.Name + "," + containerPort + "," + hostPort + "?" + ...
	// similar to how docker run -p works https://docs.docker.com/config/containers/container-networking/
	s := strings.Split(gamePortsString, "?")
	gamePortConfiguration := make([]GamePort, 0)
	gamePorts := make(map[string]string)
	for _, s2 := range s {
		if s2 == "" {
			continue
		}
		s3 := strings.Split(s2, ",")
		containerPort, err := strconv.Atoi(s3[1])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse port with portName %s, containerPort %s", s3[0], s3[2])
		}
		hostPort, err := strconv.Atoi(s3[2])
		if err != nil {
			return nil, nil, errors.Wrapf(err, "could not parse port with portName %s, hostPort %s", s3[0], s3[2])
		}

		gamePortConfiguration = append(gamePortConfiguration, GamePort{
			Name:                 s3[0],
			ServerListeningPort:  containerPort,
			ClientConnectionPort: hostPort,
		})
		gamePorts[s3[0]] = strconv.Itoa(containerPort)
	}
	return gamePorts, gamePortConfiguration, nil
}

func handleError(e error) {
	if e != nil {
		logger.Fatalf("panic because error: %s", e)
	}
}

// checkEnvOrFatal panics if the environment variable is not set
func checkEnvOrFatal(envName string, envValue string) {
	if envValue == "" {
		logger.Fatalf("Env %s is empty", envName)
	}
}

// getGameServerNameNamespaceFromEnv gets the game server name and namespace from the environment variables
// we get these variables first so we can initialize the logger
func getGameServerNameNamespaceFromEnv() {
	sessionHostId = os.Getenv("PF_GAMESERVER_NAME")
	if sessionHostId == "" {
		panic("PF_GAMESERVER_NAME is empty")
	}

	crdNamespace = os.Getenv("PF_GAMESERVER_NAMESPACE")
	if crdNamespace == "" {
		panic("PF_GAMESERVER_NAMESPACE is empty")
	}
}

// getRestEnvVariables gets the rest environment variables
func getRestEnvVariables() {
	heartbeatEndpoint = os.Getenv("HEARTBEAT_ENDPOINT")
	checkEnvOrFatal("HEARTBEAT_ENDPOINT", heartbeatEndpoint)

	gsdkConfigFilePath = os.Getenv("GSDK_CONFIG_FILE")
	checkEnvOrFatal("GSDK_CONFIG_FILE", gsdkConfigFilePath)

	sharedContentFolderPath = os.Getenv("PF_SHARED_CONTENT_FOLDER")
	checkEnvOrFatal("PF_SHARED_CONTENT_FOLDER", sharedContentFolderPath)

	certificateFolderPath = os.Getenv("CERTIFICATE_FOLDER")
	checkEnvOrFatal("CERTIFICATE_FOLDER", certificateFolderPath)

	serverLogPath = os.Getenv("PF_SERVER_LOG_DIRECTORY")
	checkEnvOrFatal("PF_SERVER_LOG_DIRECTORY", serverLogPath)

	gamePortsString = os.Getenv("PF_GAMESERVER_PORTS")
	checkEnvOrFatal("PF_GAMESERVER_PORTS", gamePortsString)

	nodeName = os.Getenv("PF_NODE_NAME")
	checkEnvOrFatal("PF_NODE_NAME", nodeName)
}
