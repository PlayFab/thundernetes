namespace thundernetes_sidecar.Model
{
    using System;
    using Newtonsoft.Json;
    using Newtonsoft.Json.Converters;

    public class HeartbeatRequest
    {
        [JsonConverter(typeof(StringEnumConverter))]
        public GameState CurrentGameState { get; set; }

        public string CurrentGameHealth { get; set; }

        public ConnectedPlayer[] CurrentPlayers { get; set; }
    }
}