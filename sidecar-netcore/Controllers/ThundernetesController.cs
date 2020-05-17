namespace thundernetes_sidecar.Controllers
{
    using System;
    using System.Net;
    using k8s;
    using k8s.Models;
    using Microsoft.Rest;
    using Newtonsoft.Json;
    using Microsoft.AspNetCore.Mvc;
    using System.Threading.Tasks;
    using thundernetes_sidecar.Model;
    using System.Collections.Generic;
    using System.Linq;

    public class ThundernetesController : ControllerBase
    {
        private static string previousGameHealth = "N/A";
        private static GameState previousGameState = GameState.Invalid;
        private static Lazy<Kubernetes> k8sClient = new Lazy<Kubernetes>(() =>
        {
            // Load from in-cluster configuration:
            var config = KubernetesClientConfiguration.InClusterConfig();
            // Use the config object to create a client.
            return new Kubernetes(config);
        });
        private static readonly string gameServerName = Environment.GetEnvironmentVariable("PF_GAMESERVER_NAME");
        private static readonly string crdGroup = "mps.playfab.com";
        private static readonly string crdVersion = "v1alpha1";
        private static readonly string crdNamespace = Environment.GetEnvironmentVariable("PF_GAMESERVER_NAMESPACE");
        private static readonly string crdPlural = "gameservers";

        private async Task<SessionDetails> GetGameServerStateFromK8s()
        {
            var sd = new SessionDetails();
            var res = await k8sClient.Value.GetNamespacedCustomObjectStatusWithHttpMessagesAsync(crdGroup, crdVersion, crdNamespace,
                crdPlural, gameServerName);
            string x = res.Response.Content.AsString();
            Console.WriteLine(x);
            dynamic y = JsonConvert.DeserializeObject(x);
            if (y.ContainsKey("status"))
            {
                dynamic status = y["status"];
                if (status.ContainsKey("state"))
                {
                    sd.State = (string)status["state"];
                }
                if (status.ContainsKey("sessionCookie"))
                {
                    sd.SessionCookie = (string)status["sessionCookie"];
                }
                if (status.ContainsKey("sessionId"))
                {
                    sd.SessionId = (string)status["sessionId"];
                }
                if (status.ContainsKey("initialPlayers"))
                {
                    sd.InitialPlayers = status["initialPlayers"].ToObject<string[]>();
                }
            }

            return sd;
        }

        [HttpPatch]
        [Route("v1/sessionHosts/{sessionHostId}")]
        public async Task<IActionResult> ProcessHeartbeatV1(
            string sessionHostId,
            [FromBody] HeartbeatRequest heartbeatRequest)
        {
            return await ProcessHeartbeat(sessionHostId, heartbeatRequest);
        }

        [HttpPost]
        [Route("v1/sessionHosts/{sessionHostId}/heartbeats")]
        public async Task<IActionResult> ProcessHeartbeat(
            string sessionHostId,
            [FromBody] HeartbeatRequest heartbeatRequest)
        {
            if (!TryValidateModelState(out string errorMessage))
            {
                return BadRequest(errorMessage);
            }

            Console.WriteLine($"Heartbeat received from sessionHostId {sessionHostId} with content {JsonConvert.SerializeObject(heartbeatRequest)}");

            // update CRD status if health has changed
            await UpdateHealthIfNeeded(heartbeatRequest);

            // when the game begins and reaches ReadyForPlayers, it will transition to a StandingBy state
            // we'll update GameServer.Status.State just once
            if (previousGameState != heartbeatRequest.CurrentGameState &&
                heartbeatRequest.CurrentGameState == GameState.StandingBy)
            {
                await TransitionStateToStandingBy();
                previousGameState = heartbeatRequest.CurrentGameState;
            }

            SessionDetails sd = await GetGameServerStateFromK8s();
            GameOperation op = GameOperation.Continue;

            if (sd.State == String.Empty) // we expect it to be empty when container is starting
            {
                op = GameOperation.Continue;
            }
            else if (sd.State == "StandingBy")
            {
                op = GameOperation.Continue;
            }
            else if (sd.State == "Active")
            {
                op = GameOperation.Active;
            }

            SessionConfig sc = new SessionConfig();
            if (!string.IsNullOrEmpty(sd.SessionId))
            {
                sc.SessionId = new Guid(sd.SessionId);
            }

            if (!string.IsNullOrEmpty(sd.SessionCookie))
            {
                sc.SessionCookie = sd.SessionCookie;
            }

            if (sd.InitialPlayers != null)
            {
                sc.InitialPlayers = new List<string>(sd.InitialPlayers);
            }

            var response = new HeartbeatResponse()
            {
                Operation = op,
                SessionConfig = sc,
            };
            return Ok(response);
        }

        private async Task UpdateHealthIfNeeded(HeartbeatRequest heartbeatRequest)
        {
            if (previousGameHealth != heartbeatRequest.CurrentGameHealth)
            {
                Console.WriteLine($"Health is different than before, updating. Old health {previousGameHealth}, new health {heartbeatRequest.CurrentGameHealth}");

                string json = $@"{{
                    ""status"": {{
                    ""health"":""{heartbeatRequest.CurrentGameHealth}""
                    }}
                }}";
                V1Patch body = new V1Patch(json, V1Patch.PatchType.MergePatch);
                var res = await k8sClient.Value.PatchNamespacedCustomObjectStatusWithHttpMessagesAsync(body,
                    crdGroup, crdVersion, crdNamespace, crdPlural, gameServerName);

                res.Response.EnsureSuccessStatusCode();

                previousGameHealth = heartbeatRequest.CurrentGameHealth;
            }
        }

        private async Task TransitionStateToStandingBy()
        {
            Console.WriteLine($"State is different than before, updating. Old state {previousGameState}, new state StandingBy");

            string json = $@"{{
                ""status"": {{
                ""state"":""{GameState.StandingBy}""
                }}
            }}";
            V1Patch body = new V1Patch(json, V1Patch.PatchType.MergePatch);
            var res = await k8sClient.Value.PatchNamespacedCustomObjectStatusWithHttpMessagesAsync(body,
                crdGroup, crdVersion, crdNamespace, crdPlural, gameServerName);

            res.Response.EnsureSuccessStatusCode();
        }

        private class SessionDetails
        {
            public string SessionId { get; set; }
            public string SessionCookie { get; set; }
            public string[] InitialPlayers { get; set; }
            public string State { get; set; }
        }

        private bool TryValidateModelState(out string errorMessage)
        {
            errorMessage = null;
            if (!ModelState.IsValid)
            {
                IEnumerable<string> keysWithInvalidValues =
                    ModelState.Where(x => x.Value.Errors.Count > 0).Select(x => x.Key);
                errorMessage = $"Invalid values specified for {string.Join(", ", keysWithInvalidValues)}.";
                Console.WriteLine(errorMessage);
                return false;
            }

            return true;
        }
    }
}