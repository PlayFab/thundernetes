---
layout: default
title: GameServer API documentation
parent: GameServer API
nav_order: 1
---

# GameServer API documentation

## Game Server Builds

### Create Game Server Build

`POST /api/v1/gameserverbuilds/`

<details markdown=block>

  Create a Game Server Build in the cluster.

  * **URL Params**

    None
  
  * **Body**

{% include code-block-start.md %}
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
    portsToExpose: Array&lt;number&gt;,
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
{% include code-block-end.md %}
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
    portsToExpose: Array&lt;number&gt;,
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 400

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
    
  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### List Game Server Builds

`GET /api/v1/gameserverbuilds/`

<details  markdown=block>

  List all the Game Server Builds in the cluster.

  * **URL Params**

    None
  
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
      portsToExpose: Array&lt;number&gt;,
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 500

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Get a Game Server Build

`GET /api/v1/gameserverbuilds/:namespace/:buildName`

<details markdown=block>

  Get a single Game Server Build from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Build

    * `buildName`: the name of the Game Server Build

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
    portsToExpose: Array&lt;number&gt;,
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}

  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Patch a Game Server Build

`PATCH /api/v1/gameserverbuilds/:namespace/:buildName`

<details markdown=block>

  Patch the standingBy and max values of a Game Server Build from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Build

    * `buildName`: the name of the Game Server Build
    
  * **Body**

{% include code-block-start.md %}
{
  "standingBy": int,
  "max": int
}
{% include code-block-end.md %}
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
    portsToExpose: Array&lt;number&gt;,
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 400

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}

  OR

  * **Code:** 404

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}

  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Delete a Game Server Build

`DELETE /api/v1/gameserverbuilds/:namespace/:buildName`

<details markdown=block>

  Delete a Game Server Build from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Build

    * `buildName`: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
{"message": "Game server build deleted"}
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}

  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

<br>

## Game Servers

### List Game Servers

`GET /api/v1/gameservers/`

<details markdown=block>

  List all the Game Servers in the cluster.

  * **URL Params**

    None
  
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 500

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### List the Game Servers from a Game Server Build

`GET /api/v1/gameserverbuilds/:namespace/:buildName/gameservers`

<details markdown=block>

  List the Game Servers owned by a specific Game Server Build.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Build

    * `buildName`: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
    
  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Get a Game Server

`GET /api/v1/gameservers/:namespace/:gameServerName`

<details markdown=block>

  Get a single Game Server from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server

    * `gameServerName`: the name of the Game Server

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}

  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Delete a Game Server

`DELETE /api/v1/gameservers/:namespace/:gameServerName`

<details markdown=block>

  Delete a Game Server from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server

    * `gameServerName`: the name of the Game Server

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
{"message": "Game server deleted"}
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
    
  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

<br>

## Game Server Details

### List the Game Server Details from a Game Server Build

`GET /api/v1/gameserverbuilds/:namespace/:buildName/gameserverdetails`

<details markdown=block>

  List the Game Server Details owned by a specific Game Server Build.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Build

    * `buildName`: the name of the Game Server Build
    
  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
{
  apiVersion: "mps.playfab.com/v1alpha1",
  kind: "GameServerDetail",
  metadata: {
    name: string,
    namespace: string
  },
  spec: {
    connectedPlayersCount: number,
    connectedPlayers: Array&lt;string&gt;
  }
}
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
    
  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>

### Get a Game Server Detail

`GET /api/v1/gameserverdetails/:namespace/:gameServerDetailName`

<details markdown=block>

  Get a single Game Server Detail from the cluster.

  * **URL Params**

    * `namespace`: the Kubernetes namespace of the Game Server Detail

    * `gameServerDetailName`: the name of the Game Server Detail

  * **Body**

    None
  
  * **Success Response**

    * **Code:** 200

      **Body:**

{% include code-block-start.md %}
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
      connectedPlayers: Array&lt;string&gt;
    }
  },
  ...
]
{% include code-block-end.md %}
  
  * **Error Response**

    * **Code:** 404

      **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
    
  OR

  * **Code:** 500

    **Body:**

{% include code-block-start.md %}
{"error": error message}
{% include code-block-end.md %}
  
</details>