package main

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pkg/errors"
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
	GamePorts                map[string]int           `json:"gamePorts"`
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
	vmId                    string
	gamePortsString         string
	sessionHostId           string
)

func main() {
	checkEnvVariables()

	gamePorts, gamePortConfiguration, err := parsePorts()
	if err != nil {
		log.Fatalf("Could not parse game ports %s", err)
	}

	buildMetadata := parseBuildMetadata()

	config := &GsdkConfig{
		HeartbeatEndpoint:   heartbeatEndpoint,
		SessionHostId:       sessionHostId,
		VmId:                vmId,
		LogFolder:           serverLogPath,
		CertificateFolder:   certificateFolderPath,
		SharedContentFolder: sharedContentFolderPath,
		BuildMetadata:       buildMetadata,
		GamePorts:           gamePorts,
		PublicIpV4Address:   "N/A", // TODO: can we have that here?
		GameServerConnectionInfo: GameServerConnectionInfo{
			PublicIpV4Address:      "N/A",
			GamePortsConfiguration: gamePortConfiguration,
		},
		FullyQualifiedDomainName: "NOT_APPLICABLE",
	}

	log.Println("Marshalling to JSON")
	configJson, err := json.Marshal(config)
	handleError(err)

	log.Println("Getting and creating folder(s)")
	folderPath := filepath.Dir(gsdkConfigFilePath)
	err = os.MkdirAll(folderPath, os.ModePerm)
	handleError(err)

	log.Printf("Creating empty GSDK JSON file %s", gsdkConfigFilePath)
	f, err := os.Create(gsdkConfigFilePath)
	handleError(err)

	log.Printf("Saving GSDK JSON to file %s", gsdkConfigFilePath)
	_, err = f.Write(configJson)
	handleError(err)
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

func parsePorts() (map[string]int, []GamePort, error) {
	// format is port.Name + "," + containerPort + "," + hostPort + "?" + ...
	// similar to how docker run -p works https://docs.docker.com/config/containers/container-networking/
	s := strings.Split(gamePortsString, "?")
	gamePortConfiguration := make([]GamePort, 0)
	gamePorts := make(map[string]int)
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
		gamePorts[s3[0]] = containerPort
	}
	return gamePorts, gamePortConfiguration, nil
}

func handleError(e error) {
	if e != nil {
		panic(e)
	}
}

func checkEnvOrFail(envName string, envValue string) {
	if envValue == "" {
		log.Fatalf("Env %s is empty", envName)
	}
}

func checkEnvVariables() {
	heartbeatEndpoint = os.Getenv("HEARTBEAT_ENDPOINT")
	checkEnvOrFail("HEARTBEAT_ENDPOINT", heartbeatEndpoint)

	gsdkConfigFilePath = os.Getenv("GSDK_CONFIG_FILE")
	checkEnvOrFail("GSDK_CONFIG_FILE", gsdkConfigFilePath)

	sharedContentFolderPath = os.Getenv("PF_SHARED_CONTENT_FOLDER")
	checkEnvOrFail("PF_SHARED_CONTENT_FOLDER", sharedContentFolderPath)

	certificateFolderPath = os.Getenv("CERTIFICATE_FOLDER")
	checkEnvOrFail("CERTIFICATE_FOLDER", certificateFolderPath)

	serverLogPath = os.Getenv("PF_SERVER_LOG_DIRECTORY")
	checkEnvOrFail("PF_SERVER_LOG_DIRECTORY", serverLogPath)

	vmId = os.Getenv("PF_VM_ID")
	checkEnvOrFail("PF_VM_ID", vmId)

	gamePortsString = os.Getenv("PF_GAMESERVER_PORTS")
	checkEnvOrFail("PF_GAMESERVER_PORTS", gamePortsString)

	sessionHostId = os.Getenv("PF_GAMESERVER_NAME")
	checkEnvOrFail("PF_GAMESERVER_NAME", sessionHostId)
}
