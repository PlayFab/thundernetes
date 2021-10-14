package main;

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
);

func main () {
	args := os.Args;
	
	if len(args) <= 1 {
		fmt.Println("Usage of the allocator tool");
		fmt.Println("\tallocate #Initialize a server with the given paramaters");
		fmt.Println("\tlist # Returns the available Game Servers on Standby status");
	} else if strings.Compare(args[1], "allocate") == 0 {
		fmt.Println("Beginning the allocate process");

		cmd := exec.Command("kubectl","get","svc","-n","thundernetes-system","thundernetes-controller-manager",
			"-o","jsonpath='{.status.loadBalancer.ingress[0].ip}'");
		var ip string;
		reqBody, err := json.Marshal(map[string] string {
			"buildID": "85ffe8da-c82f-4035-86c5-9d2b5f42d6f6",
			"sessionID": "ac1b7082-d811-47a7-89ae-fe1a9c48a6da",
		});
		

		if err != nil {
			log.Fatal(err);
		}

		output, err := cmd.CombinedOutput();

		if err != nil {
			log.Fatal(string(output));
		}

		
		if len(output) < 3 { // basically if we don't have a valid IP
			ip = "http://127.0.0.1:5000/api/v1/allocate";
		} else {
			ip = string(output);
		}

		resp, err := http.Post(ip, "application/json", bytes.NewBuffer(reqBody));

		if err != nil {
			Fatal.Println(err);
		}

		defer resp.Body.Close();

		body, err := ioutil.ReadAll(resp.Body);

		if err != nil {
			log.Println(err);
		}

		log.Println(string(body));
		
	} else if strings.Compare(args[1], "list") == 0 {
		fmt.Println("Listing the available game servers");
		cmd := exec.Command("kubectl", "get", "gs");
		output, err := cmd.CombinedOutput();

		if err != nil {
			fmt.Println(string(output));
			log.Fatal("Error while fetching the servers: ", err);
			fmt.Println("Please, make sure you have your cluster configured properly");
		}

		fmt.Println(string(output));
	} else {
		fmt.Println("Sorry, but the commad "+args[1]+" is not recognized");
	}

	fmt.Println("\nThanks for using the thundernetes allocator tool");
	
}