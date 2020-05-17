namespace thundernetes_sidecar.Model
{
    using System;
    using System.Collections.Generic;
    using Newtonsoft.Json;

    internal class SessionConfig
    {
        [JsonProperty(PropertyName = "sessionId")]
        public Guid SessionId { get; set; }

        [JsonProperty(PropertyName = "sessionCookie")]
        public string SessionCookie { get; set; }

        [JsonProperty(PropertyName = "initialPlayers")]
        public List<string> InitialPlayers { get; set; }

        [JsonProperty(PropertyName = "metadata")]
        public Dictionary<string, string> Metadata { get; set; }
    }
}