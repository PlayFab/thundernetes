---
layout: default
title: How to use Thundernetes UI
parent: Thundernetes UI
nav_order: 1
---

# How to use Thundernetes UI

## How to config the app
The app needs a file called `config.js` with the endpoints to the GameServer API and to the Thundernetes manager (this is only to allocate game servers). Inside the file you need to define a variable called `clusters` with the following structure:

{% include code-block-start.md %}
var clusters = {
  "cluster1": {
    "api": "http://{cluster1_api_IP}:5001/api/v1/",
    "allocate": "http://{cluster1_manager_IP}:5000/api/v1/allocate"
  },
  "cluster2": {
    "api": "http://{cluster2_api_IP}:5001/api/v1/",
    "allocate": "http://{cluster2_manager_IP}:5000/api/v1/allocate"
  }
}
 {% include code-block-end.md %}

## How to run locally
If you want to run the project locally, first you need to install [Node.js](https://nodejs.org/en/download/). Then clone the project:

{% include code-block-start.md %}
git clone https://github.com/PlayFab/thundernetes-ui.git
{% include code-block-end.md %}

And install the dependencies:

{% include code-block-start.md %}
npm install
{% include code-block-end.md %}

After this, you can create the `config.js` file inside the public folder, then you can simply run the app with the `npm start` command. This will start a server and open a browser to `http://localhost:3000`.

## How to run using the Docker image

You can also run the Docker container image, all you have to do is mount a volume to pass your `config.js` file to the app, you can do this like this:

{% include code-block-start.md %}
docker run -d -p 80:80 -v [path to your config.js]:/usr/share/nginx/html/config.js ghcr.io/playfab/thundernetes-ui:[current tag]
{% include code-block-end.md %}
