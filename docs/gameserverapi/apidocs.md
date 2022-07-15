---
layout: default
title: GameServer API documentation
parent: GameServer API
nav_order: 1
---

# GameServer API documentation

## Game Server Builds

### Create Game Server Build

```POST /api/v1/gameserverbuilds/```

<details markdown=block>

  Create a Game Server Build in the cluster.

  * **URL Params**

    None
  
  * **Body**

    ```js
    {
      apiVersion: "mps.playfab.com/v1alpha1",
      kind: "GameServerBuild",
      metadata: {
        name: string,
        namespace: string | undefined,
      },
      spec: {
        buildID: string,
        standingBy: number,
        max: number,
        portsToExpose: Array<number>,
        crashesToMarkUnhealthy: number | undefined,
        template: any
      },
      status: {
        currentActive: number,
        currentStandingBy: number,
        crashesCount: number,
        currentPending: number,
        currentInitializing: number,
        health: string,
        currentStandingByReadyDesired: string,
      }
    }
    ```
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {
        apiVersion: "mps.playfab.com/v1alpha1",
        kind: "GameServerBuild",
        metadata: {
          name: string,
          namespace: string,
        },
        spec: {
          buildID: string,
          standingBy: number,
          max: number,
          portsToExpose: Array<number>,
          crashesToMarkUnhealthy: number | undefined,
          template: any
        },
        status: {
          currentActive: number,
          currentStandingBy: number,
          crashesCount: number,
          currentPending: number,
          currentInitializing: number,
          health: string,
          currentStandingByReadyDesired: string,
        }
      }
      ```
  
  * **Error Response**

    * **Code:** 400

      **Body:**

      ```js
      {"error": error message}
      ```
    
    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### List Game Server Builds

```GET /api/v1/gameserverbuilds/```

<details  markdown=block>

  List all the Game Server Builds in the cluster.

  * **URL Params**

    None
  
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      [
        {
          apiVersion: "mps.playfab.com/v1alpha1",
          kind: "GameServerBuild",
          metadata: {
            name: string,
            namespace: string,
          },
          spec: {
            buildID: string,
            standingBy: number,
            max: number,
            portsToExpose: Array<number>,
            crashesToMarkUnhealthy: number | undefined,
            template: any
          },
          status: {
            currentActive: number,
            currentStandingBy: number,
            crashesCount: number,
            currentPending: number,
            currentInitializing: number,
            health: string,
            currentStandingByReadyDesired: string,
          }
        },
        ...
      ]
      ```
  
  * **Error Response**

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Get a Game Server Build

```GET /api/v1/gameserverbuilds/:namespace/:buildName```

<details markdown=block>

  Get a single Game Server Build from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Build

    * ```buildName```: the name of the Game Server Build

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {
        apiVersion: "mps.playfab.com/v1alpha1",
        kind: "GameServerBuild",
        metadata: {
          name: string,
          namespace: string,
        },
        spec: {
          buildID: string,
          standingBy: number,
          max: number,
          portsToExpose: Array<number>,
          crashesToMarkUnhealthy: number | undefined,
          template: any
        },
        status: {
          currentActive: number,
          currentStandingBy: number,
          crashesCount: number,
          currentPending: number,
          currentInitializing: number,
          health: string,
          currentStandingByReadyDesired: string,
        }
      }
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```

    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Patch a Game Server Build

```PATCH /api/v1/gameserverbuilds/:namespace/:buildName```

<details markdown=block>

  Patch the standingBy and max values of a Game Server Build from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Build

    * ```buildName```: the name of the Game Server Build
    
  * **Body**

    ```js
    {
      "standingBy": int,
      "max": int
    }
    ```
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {
        apiVersion: "mps.playfab.com/v1alpha1",
        kind: "GameServerBuild",
        metadata: {
          name: string,
          namespace: string,
        },
        spec: {
          buildID: string,
          standingBy: number,
          max: number,
          portsToExpose: Array<number>,
          crashesToMarkUnhealthy: number | undefined,
          template: any
        },
        status: {
          currentActive: number,
          currentStandingBy: number,
          crashesCount: number,
          currentPending: number,
          currentInitializing: number,
          health: string,
          currentStandingByReadyDesired: string,
        }
      }
      ```
  
  * **Error Response**

    * **Code:** 400

      **Body:**

      ```js
      {"error": error message}
      ```

    OR

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```

    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Delete a Game Server Build

```DELETE /api/v1/gameserverbuilds/:namespace/:buildName```

<details markdown=block>

  Delete a Game Server Build from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Build

    * ```buildName```: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {"message": "Game server build deleted"}
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```

    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

<br>

## Game Servers

### List Game Servers

```GET /api/v1/gameservers/```

<details markdown=block>

  List all the Game Servers in the cluster.

  * **URL Params**

    None
  
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      [
        {
          apiVersion: "mps.playfab.com/v1alpha1",
          kind: "GameServer",
          metadata: {
            name: string,
            namespace: string
          },
          status: {
            state: string,
            health: string,
            publicIP: string,
            ports: string,
            nodeName: string
          }
        },
        ...
      ]
      ```
  
  * **Error Response**

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### List the Game Servers from a Game Server Build

```GET /api/v1/gameserverbuilds/:namespace/:buildName/gameservers```

<details markdown=block>

  List the Game Servers owned by a specific Game Server Build.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Build

    * ```buildName```: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      [
        {
          apiVersion: "mps.playfab.com/v1alpha1",
          kind: "GameServer",
          metadata: {
            name: string,
            namespace: string
          },
          status: {
            state: string,
            health: string,
            publicIP: string,
            ports: string,
            nodeName: string
          }
        },
        ...
      ]
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```
    
    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Get a Game Server

```GET /api/v1/gameservers/:namespace/:gameServerName```

<details markdown=block>

  Get a single Game Server from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server

    * ```gameServerName```: the name of the Game Server

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {
        apiVersion: "mps.playfab.com/v1alpha1",
        kind: "GameServer",
        metadata: {
          name: string,
          namespace: string
        },
        status: {
          state: string,
          health: string,
          publicIP: string,
          ports: string,
          nodeName: string
        }
      }
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```

    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Delete a Game Server

```DELETE /api/v1/gameservers/:namespace/:gameServerName```

<details markdown=block>

  Delete a Game Server from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server

    * ```gameServerName```: the name of the Game Server

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {"message": "Game server deleted"}
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```
    
    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

<br>

## Game Server Details

### List the Game Server Details from a Game Server Build

```GET /api/v1/gameserverbuilds/:namespace/:buildName/gameserverdetails```

<details markdown=block>

  List the Game Server Details owned by a specific Game Server Build.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Build

    * ```buildName```: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      {
        apiVersion: "mps.playfab.com/v1alpha1",
        kind: "GameServerDetail",
        metadata: {
          name: string,
          namespace: string
        },
        spec: {
          connectedPlayersCount: number,
          connectedPlayers: Array<string>
        }
      }
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```
    
    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>

### Get a Game Server Detail

```GET /api/v1/gameserverdetails/:namespace/:gameServerDetailName```

<details markdown=block>

  Get a single Game Server Detail from the cluster.

  * **URL Params**

    * ```namespace```: the Kubernetes namespace of the Game Server Detail

    * ```gameServerDetailName```: the name of the Game Server Detail

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

      ```js
      [
        {
          apiVersion: "mps.playfab.com/v1alpha1",
          kind: "GameServerDetail",
          metadata: {
            name: string,
            namespace: string
          },
          spec: {
            connectedPlayersCount: number,
            connectedPlayers: Array<string>
          }
        },
        ...
      ]
      ```
  
  * **Error Response**

    * **Code:** 404

      **Body:**

      ```js
      {"error": error message}
      ```
    
    OR

    * **Code:** 500

      **Body:**

      ```js
      {"error": error message}
      ```
  
</details>
