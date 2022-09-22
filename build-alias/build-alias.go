package main;

import (
	"bytes"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"log"
	"net/http"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
)

type GameServerBuild struct {
	BuildId string
	Weight int
}

func main() {
	args := os.Args;
	fileName := "thundernetes-buildalias.yaml";
	helpMessage := "\n\tThundernetes build alias feature\n\n" +
	"1) create - generates a new build alias with the name provided." +
	" Each alias is made of one or more pairs of build id and weights (priority of assignment for allocation). Template for using:" +
	"\n\n\tbuild-alias create <alias-name> <buildId1> <weight1> <buildId2> <weight2> ... <buildIdN> <weightN>\n\n" +
	"2) allocate - create a new game server instance for the build alias and session id provided. Template for using\n\n\t" +
	"build-alias allocate <alias-name> <sessionId>\n\n" +
	"3) help - Displays this message and exit";

	if len(args) == 1 {
		fmt.Println(helpMessage);
		return;
	}

	switch(args[1]) {

	case "create":
		CreateBuildAlias(args, fileName);

	case "update":
		fmt.Println("Updating the build alias " + args[2]);

	case "allocate":
		AllocateForBuildAlias(args, fileName);

	case "help":
		fmt.Println(helpMessage);

	default:
		fmt.Println("Sorry, but the command "+ args[1] + " is not recognized.");
	}
}

func CreateBuildAlias(args []string, fileName string) {
	if len(args) < 5 {
		log.Fatal("Not enough arguments were provided.");
	}

	fmt.Println("Creating the build alias: " + args[2]);

	buildName := args[2];
	aliasMap := map[string]map[string]int{};

	aliasMap[buildName] = make(map[string]int);

	for i := 3; i < len(args) - 1; i += 2 {
		buildId := args[i];

		if weight, err := strconv.Atoi(args[i+1]); err == nil {
			aliasMap[buildName][buildId] = weight;
		} else {
			log.Fatal("Invalid weight " + args[i+1] + " provided for buildId: " + buildId + ". ", err);
		}
	}

	yamlData, err := yaml.Marshal(&aliasMap);

	if err != nil {
		log.Fatal("Error while Marshaling. \n", err);
	}

	f, err := os.OpenFile(fileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644);

	if err != nil {
		log.Fatal("Error when creating the alias", err);
	}

	if _, err := f.Write(yamlData); err != nil {
		f.Close();
		log.Fatal(err);
	}

	if err := f.Close(); err != nil {
		log.Fatal(err);
	}

	fmt.Println("Build alias " + buildName + " created successfully.");
}

func AllocateForBuildAlias(args []string, fileName string) {
	if len(args) < 4 {
		log.Fatal("Not enough arguments were provided.");
	}

	buildName := args[2];
	sessionId := args[3];
	var ipToAllocate string;

	cmd := exec.Command("kubectl","get","svc","-n","thundernetes-system","thundernetes-controller-manager",
			"-o","jsonpath='{.status.loadBalancer.ingress[0].ip}'");
	commandOutput, err := cmd.CombinedOutput();

	fmt.Println("Attempting to generate the IP address to allocate");
	if err != nil {
		log.Fatal("Error while trying to generate the IP, ", err);
	}

	if len(commandOutput) < 3 { // basically if we don't have a valid IP
		// We will allocate locally
		fmt.Println("Not able to generate the IP to allocate. Will use local environment");
		ipToAllocate = "http://127.0.0.1:5000/api/v1/allocate";
	} else {
		ipToAllocate = string(commandOutput);
		fmt.Println("IP", ipToAllocate, "to allocate generated successfully");

		// We need to remove the '' at the begining and end of the IP address
		ipToAllocate = strings.Trim(ipToAllocate, "'");
		ipToAllocate = "http://" + ipToAllocate + ":5000/api/v1/allocate";
	}

	fmt.Println("Allocating a Game Server for " + buildName);

	aliasMap := map[string]map[string]int{};

	yamlFile, err := os.ReadFile(fileName);

	if err != nil {
		log.Fatal("Error while opening the file "+ fileName +". \n", err);
	}

	err = yaml.Unmarshal(yamlFile, &aliasMap);

	if err != nil {
		log.Fatal("Error while unmarshaling the file "+ fileName +". \n", err);
	}

	buildIds := aliasMap[buildName];
	gameServers := []GameServerBuild{};

	// A naive implementation of map to a slice for GameServerBuilds
	for k, v := range buildIds {
		gameServers = append(gameServers, GameServerBuild{BuildId: k, Weight: v});
	}

	// This is to override the default ascending behavior of sort library,
	// we declare a helper function to tell explicitely how to order our
	// slice (in this case in descending order by weight)
	sort.Slice(gameServers, func(i, j int) bool {
		return gameServers[i].Weight > gameServers[j].Weight;
	});

	serverAllocated := false;
	// Here, we iterate linearly since the elements were previously ordered and we're
	// giving priority to the builds with greater weights
	for i:= 0; i < len(gameServers); i++ {
		fmt.Println("Attempting to allocate...");

		reqBody := map[string]string{"buildID": gameServers[i].BuildId, "sessionID": sessionId};
		jsonData, err := json.Marshal(reqBody);

		if err != nil {
			log.Fatal("Error while marshaling the request. \n", err);
		}

		resp, err := http.Post(ipToAllocate, "application/json", bytes.NewBuffer(jsonData));

		if err != nil {
			log.Fatal("Error while allocating the game server. \n", err);
		}

		var result map[string]interface{};
		json.NewDecoder(resp.Body).Decode(&result);

		// The post request returned a success as result code
		if resp.StatusCode == 200 {
			fmt.Println("Game server allocated successfully at IP Address: ", result["IPV4Address"], 
			            " with ports: ", result["Ports"]);

			serverAllocated = true;
			resp.Body.Close();
			break;
		}

		fmt.Println("Allocation on server unsuccessful. Server Result code: ", resp.StatusCode);
		resp.Body.Close();
	}
	
	if !serverAllocated {
		fmt.Println("No server was available to allocate for build id", buildName);
	}
}