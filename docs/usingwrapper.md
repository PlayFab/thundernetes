# Using a wrapper utility 

## Mpswrapper

Your game server needs to be integrated with the Game Server SDK ([GSDK](https://github.com/PlayFab/gsdk)) in order to work with thundernetes. However, you can use a wrapper application which:

- integrates with GSDK
- starts and monitors ("wraps") your game server executable

We have built such a utility (called 'mpswrapper') [here](https://github.com/PlayFab/MpsSamples/tree/master/wrappingGsdk). Mpswrapper is also published as container image on GitHub Container Registry [here](https://github.com/PlayFab/MpsSamples/pkgs/container/mpswrapper) which should be used as the container image in your GameServerBuild definition.

Your game server needs to be built and packaged in a place where your Kubernetes cluster can access it. Kubernetes uses [volumes](https://kubernetes.io/docs/concepts/storage/volumes/) to access data external to the Pod. For this sample, you will use [Azure Files](https://azure.microsoft.com/en-us/services/storage/files/) storage to place your game server files but you are free to use the storage service of your choice.

> Azure Files **Premium** is recommended for optimal performance.

> Your game server should *NOT* be integrated with GSDK, since the mpswrapper application is already integrated. 

As soon as you create an Azure Files account, create a Files share and put your files there, you should make sure that Kubernetes knows how to authenticate to your Azure Storage account. One way to do that is create a Kubernetes secret using the below script:

```bash
SECRET_NAME=azure-secret # name of the Kubernetes secret object
SHARE_NAME=thundernetesshare # name of the Azure Files share
ACCOUNT_KEY=YOUR_ACCOUNT_KEY # key for your Azure Storage account
kubectl create secret generic $SECRET_NAME --from-literal=azurestorageaccountname=$SHARE_NAME --from-literal=azurestorageaccountkey=$ACCOUNT_KEY
```

As soon as the secret is created, you are ready to create your thundernetes GameServerBuild. The `containers' part of the YAML should look like this:

```YAML
containers:
  - image: ghcr.io/playfab/mpswrapper:0.1.0 
    name: mpswrapper
    command: ["/bin/bash", "-c", "chmod +x /assets/fakegame && ./wrapper -g /assets/fakegame"] # we use /assets since this is the folder specified on volumeMounts.mountPath below
    ports:
    - containerPort: 80 # your game server port
      protocol: TCP # your game server port protocol
      name: gameport # required field
    volumeMounts:
    - name: azure # must be the same as volumes.name below
      mountPath: /assets # the path that the files will be mounted
volumes:
- name: azure # must be the name as volumeMounts.name below
  azureFile:
    secretName: azure-secret
    shareName: fakegame # the share name of the Azure Files account where you placed your game files
    readOnly: false
```

### Links

- Mpswrapper code is [here](https://github.com/PlayFab/MpsSamples/tree/master/wrappingGsdk) and the latest container image version can be found [here](https://github.com/PlayFab/MpsSamples/pkgs/container/mpswrapper)
- Instructions on how to mount an Azure Files share on your Kubernetes Pods can be found [here](https://docs.microsoft.com/en-us/azure/aks/azure-files-volume)
- You can find a sample YAML file [here](../samples/fileshare/sample.yaml)